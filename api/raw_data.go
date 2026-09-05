package api

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"syscall"
	"time"

	"kvcache/storage"
)

// 裸 TCP 数据面（读专用）：大 value 走 sendfile 从 page cache 直发 socket，
// 不经过用户态，也没有 gRPC 的 16KB 帧拼装拷贝——pprof 实测服务端读路径
// 72% CPU 在 syscall（文件读+socket 写各一次内核拷贝），sendfile 把它消掉。
//
// 协议（大端，连接持久，每连接一次一个请求）：
//   请求:  [4B keyLen][key]
//   响应:  [1B status][8B bodyLen][body]
//   status: 0=ok，1=not found，2=error（body 为错误信息）
//
// 与 gRPC/HTTP 服务并存，默认监听 gRPC 端口 +2。
type RawDataServer struct {
	store storage.Storage
	ln    net.Listener
	done  chan struct{}
}

const (
	rawStatusOK       = 0
	rawStatusNotFound = 1
	rawStatusError    = 2
)

// StartRawDataServer 在 addr 上启动裸 TCP 数据面服务
func StartRawDataServer(addr string, store storage.Storage) (*RawDataServer, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := &RawDataServer{store: store, ln: ln, done: make(chan struct{})}
	go s.acceptLoop()
	log.Printf("Raw data server (sendfile) started on %s", addr)
	return s, nil
}

func (s *RawDataServer) Close() {
	close(s.done)
	s.ln.Close()
}

// Addr 返回实际监听地址（如 :0 自动分配后的真实端口）
func (s *RawDataServer) Addr() string {
	return s.ln.Addr().String()
}

func (s *RawDataServer) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			select {
			case <-s.done:
				return
			default:
				log.Printf("raw data accept error: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		if tc, ok := conn.(*net.TCPConn); ok {
			tc.SetNoDelay(true)
		}
		go s.serveConn(conn)
	}
}

func (s *RawDataServer) serveConn(conn net.Conn) {
	defer conn.Close()

	hdr := make([]byte, 4)
	for {
		// 请求头：4B keyLen + key
		if _, err := io.ReadFull(conn, hdr); err != nil {
			return // 对端关闭或读失败，直接断开
		}
		keyLen := binary.BigEndian.Uint32(hdr)
		if keyLen == 0 || keyLen > 64*1024 {
			return
		}
		key := make([]byte, keyLen)
		if _, err := io.ReadFull(conn, key); err != nil {
			return
		}

		if err := s.handleGet(conn, key); err != nil {
			log.Printf("raw data conn %s closed: %v", conn.RemoteAddr(), err)
			return
		}
	}
}

// handleGet 处理一次读请求；返回非 nil error 表示连接级失败
func (s *RawDataServer) handleGet(conn net.Conn, key []byte) error {
	filePath, inline, found, err := s.store.RawLookup(key)

	respHdr := make([]byte, 9)
	switch {
	case err != nil:
		msg := []byte(err.Error())
		respHdr[0] = rawStatusError
		binary.BigEndian.PutUint64(respHdr[1:], uint64(len(msg)))
		if _, werr := conn.Write(respHdr); werr != nil {
			return werr
		}
		_, werr := conn.Write(msg)
		return werr
	case !found:
		respHdr[0] = rawStatusNotFound
		binary.BigEndian.PutUint64(respHdr[1:], 0)
		_, werr := conn.Write(respHdr)
		return werr
	case inline != nil:
		respHdr[0] = rawStatusOK
		binary.BigEndian.PutUint64(respHdr[1:], uint64(len(inline)))
		if _, werr := conn.Write(respHdr); werr != nil {
			return werr
		}
		_, werr := conn.Write(inline)
		return werr
	}

	return s.sendFile(conn, filePath, respHdr)
}

// sendFile 先发响应头，再 sendfile 整个文件（page cache → socket，零用户态拷贝）
func (s *RawDataServer) sendFile(conn net.Conn, path string, respHdr []byte) error {
	f, err := os.Open(path)
	if err != nil {
		msg := []byte(fmt.Sprintf("open value file: %v", err))
		respHdr[0] = rawStatusError
		binary.BigEndian.PutUint64(respHdr[1:], uint64(len(msg)))
		if _, werr := conn.Write(respHdr); werr != nil {
			return werr
		}
		_, werr := conn.Write(msg)
		return werr
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %v", path, err)
	}
	size := fi.Size()

	// 注意：不要在这里加 fadvise(WILLNEED/SEQUENTIAL)——实测对 4MB 单文件
	// 顺序读反而是轻微负收益（9.53/9.54 vs 9.81 GB/s），预读窗口由块设备
	// read_ahead_kb 控制即可（部署时要求设为 4096，见 README/部署文档）。

	respHdr[0] = rawStatusOK
	binary.BigEndian.PutUint64(respHdr[1:], uint64(size))
	if _, err := conn.Write(respHdr); err != nil {
		return err
	}

	tc, ok := conn.(*net.TCPConn)
	if !ok {
		// 非 TCP（理论上不会），回退到用户态拷贝
		_, err := io.Copy(conn, f)
		return err
	}

	raw, err := tc.SyscallConn()
	if err != nil {
		_, err := io.Copy(conn, f)
		return err
	}

	var offset int64 = 0
	inFD := int(f.Fd())
	var serr error
	werr := raw.Write(func(outFD uintptr) bool {
		// 单次 sendfile 上限 0x7ffff000 且 socket 缓冲满会短写，循环到发完。
		// 注意 offset 由 sendfile 内部更新（指向最后一个已读字节之后），
		// 调用方不得再手动累加返回值。
		for offset < size {
			_, serr = syscall.Sendfile(int(outFD), inFD, &offset, int(size-offset))
			if serr == syscall.EAGAIN {
				return false // socket 缓冲满，等 netpoller 可写事件后重试
			}
			if serr != nil {
				return true
			}
		}
		return true
	})
	if werr != nil {
		return werr
	}
	if serr != nil {
		return serr
	}
	if offset != size {
		return fmt.Errorf("sendfile short write: %d/%d", offset, size)
	}
	return nil
}
