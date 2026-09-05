package proto

import (
	"bytes"
	"testing"

	"google.golang.org/grpc/mem"
)

func TestRawCodecRoundTrip(t *testing.T) {
	c := rawCodec{}

	// SetRequest
	setReq := &SetRequest{Key: []byte("k1"), Value: bytes.Repeat([]byte("x"), 4<<20)}
	bs, err := c.Marshal(setReq)
	if err != nil {
		t.Fatal(err)
	}
	var setReq2 SetRequest
	if err := c.Unmarshal(bs, &setReq2); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(setReq2.Key, setReq.Key) || !bytes.Equal(setReq2.Value, setReq.Value) {
		t.Fatalf("SetRequest mismatch: key=%q valueLen=%d", setReq2.Key, len(setReq2.Value))
	}

	// GetRequest
	getReq := &GetRequest{Key: []byte("hello")}
	bs, _ = c.Marshal(getReq)
	var getReq2 GetRequest
	if err := c.Unmarshal(bs, &getReq2); err != nil {
		t.Fatal(err)
	}
	if string(getReq2.Key) != "hello" {
		t.Fatalf("GetRequest key=%q", getReq2.Key)
	}

	// SetResponse（带错误）
	setResp := &SetResponse{Success: false, Error: "boom"}
	bs, _ = c.Marshal(setResp)
	var setResp2 SetResponse
	if err := c.Unmarshal(bs, &setResp2); err != nil {
		t.Fatal(err)
	}
	if setResp2.Success || setResp2.Error != "boom" {
		t.Fatalf("SetResponse mismatch: %+v", setResp2)
	}

	// GetResponse（found + 空 error + value）
	getResp := &GetResponse{Found: true, Value: []byte("v123")}
	bs, _ = c.Marshal(getResp)
	var getResp2 GetResponse
	if err := c.Unmarshal(bs, &getResp2); err != nil {
		t.Fatal(err)
	}
	if !getResp2.Found || getResp2.Error != "" || !bytes.Equal(getResp2.Value, []byte("v123")) {
		t.Fatalf("GetResponse mismatch: %+v", getResp2)
	}

	// 不支持的类型应报错
	if _, err := c.Marshal(&DeleteRequest{}); err == nil {
		t.Fatal("expected error for unsupported type")
	}
	if err := c.Unmarshal(mem.BufferSlice{}, &DeleteRequest{}); err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestRawCodecTruncated(t *testing.T) {
	c := rawCodec{}
	short := mem.SliceBuffer([]byte{1, 2})
	var req SetRequest
	if err := c.Unmarshal(mem.BufferSlice{short}, &req); err == nil {
		t.Fatal("expected error for short frame")
	}
}
