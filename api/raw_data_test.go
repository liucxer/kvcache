package api

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kvcache/config"
	"kvcache/storage"
)

// fakeStorage 仅实现 RawLookup 有逻辑，其余方法满足 storage.Storage 接口
type fakeStorage struct {
	files   map[string]string
	inlines map[string][]byte
}

func (f *fakeStorage) RawLookup(key []byte) (string, []byte, bool, error) {
	k := string(key)
	if p, ok := f.files[k]; ok {
		return p, nil, true, nil
	}
	if v, ok := f.inlines[k]; ok {
		return "", v, true, nil
	}
	return "", nil, false, nil
}

func (f *fakeStorage) Set(key, value []byte) error                          { return nil }
func (f *fakeStorage) Get(key []byte) ([]byte, bool, error)                 { return nil, false, nil }
func (f *fakeStorage) Delete(key []byte) error                              { return nil }
func (f *fakeStorage) Scan(prefix []byte, limit int) ([][]byte, error)      { return nil, nil }
func (f *fakeStorage) ScanWithValues(prefix []byte, limit int) (map[string][]byte, error) {
	return nil, nil
}
func (f *fakeStorage) MSet(keyValues map[string][]byte) error               { return nil }
func (f *fakeStorage) MGet(keys [][]byte) (map[string][]byte, error)        { return nil, nil }
func (f *fakeStorage) MDelete(keys [][]byte) error                          { return nil }
func (f *fakeStorage) GetConfig() (*config.Config, error)                   { return nil, nil }
func (f *fakeStorage) UpdateConfig(cfg *config.Config) error                { return nil }
func (f *fakeStorage) Start() error                                         { return nil }
func (f *fakeStorage) Stop() error                                          { return nil }
func (f *fakeStorage) GetDiskCapacity() (int64, int64, int64, error)        { return 0, 0, 0, nil }
func (f *fakeStorage) StartEvictionManager() error                          { return nil }
func (f *fakeStorage) StopEvictionManager() error                           { return nil }

var _ storage.Storage = (*fakeStorage)(nil)

func rawRequest(t *testing.T, conn net.Conn, key string) (byte, []byte) {
	t.Helper()
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(key)))
	if _, err := conn.Write(hdr[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte(key)); err != nil {
		t.Fatal(err)
	}
	var respHdr [9]byte
	if _, err := io.ReadFull(conn, respHdr[:]); err != nil {
		t.Fatal(err)
	}
	bodyLen := binary.BigEndian.Uint64(respHdr[1:])
	body := make([]byte, bodyLen)
	if _, err := io.ReadFull(conn, body); err != nil {
		t.Fatal(err)
	}
	return respHdr[0], body
}

func TestRawDataServer(t *testing.T) {
	// 8MB 落盘文件（走 sendfile 路径）
	dir := t.TempDir()
	bigValue := make([]byte, 8<<20)
	if _, err := rand.Read(bigValue); err != nil {
		t.Fatal(err)
	}
	bigPath := filepath.Join(dir, "big.bin")
	if err := os.WriteFile(bigPath, bigValue, 0644); err != nil {
		t.Fatal(err)
	}

	fs := &fakeStorage{
		files:   map[string]string{"big": bigPath},
		inlines: map[string][]byte{"small": []byte("inline-value")},
	}

	srv, err := StartRawDataServer("127.0.0.1:0", fs)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	conn, err := net.DialTimeout("tcp", srv.Addr(), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// 1. 大 value：sendfile 路径，逐字节一致
	status, body := rawRequest(t, conn, "big")
	if status != rawStatusOK {
		t.Fatalf("big: status=%d body=%q", status, body)
	}
	if !bytes.Equal(body, bigValue) {
		t.Fatalf("big: got %d bytes, content mismatch: %v", len(body), !bytes.Equal(body, bigValue))
	}

	// 2. 内联小 value
	status, body = rawRequest(t, conn, "small")
	if status != rawStatusOK || string(body) != "inline-value" {
		t.Fatalf("small: status=%d body=%q", status, body)
	}

	// 3. 未找到
	status, body = rawRequest(t, conn, "missing")
	if status != rawStatusNotFound || len(body) != 0 {
		t.Fatalf("missing: status=%d bodyLen=%d", status, len(body))
	}

	// 4. 同连接持续可用（协议状态未损坏）
	status, body = rawRequest(t, conn, "big")
	if status != rawStatusOK || len(body) != len(bigValue) {
		t.Fatalf("big again: status=%d bodyLen=%d", status, len(body))
	}
}
