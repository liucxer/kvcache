// Package nefsagent 实现高性能长连接 RPC 协议，替代 HTTP/1.0 的 proxy.py。
//
// 设计目标：消灭每次命令的 TCP 握手开销（~7ms），用单条持久 TCP 连接
// + length-prefixed 二进制帧 + reqID 多路复用，把单命令 RTT 从 ~12ms
// 降到纯网络 RTT ~5ms。
//
// 帧格式（大端）：
//
//	[4B total_len][8B reqID][1B msgType][payload]
//
//	total_len = reqID(8) + msgType(1) + len(payload)
//
// 消息类型：
//
//	0x01 Auth 请求/响应     payload = AuthReq / AuthResp (JSON)
//	0x02 Exec 请求           payload = ExecReq (JSON)
//	0x03 Exec 响应           payload = ExecResp (JSON)
//	0x04 Upload 请求         payload = UploadReq (JSON) + raw 文件内容（JSON 后接）
//	0x05 Upload 响应         payload = UploadResp (JSON)
//	0x06 Download 请求       payload = DownloadReq (JSON)
//	0x07 Download 响应       payload = DownloadResp (JSON) + raw 文件内容
//	0x08 Health 请求         payload = 空
//	0x09 Health 响应         payload = HealthResp (JSON)
//	0xFF Error 响应         payload = ErrorResp (JSON)，reqID 对应当前请求
//
// 鉴权：连接建立后第一条帧必须是 Auth 请求，服务端验证 token 后才允许
// 其他操作；失败则断开连接。
package nefsagent

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// 消息类型
const (
	MsgAuthReq     byte = 0x01
	MsgAuthResp    byte = 0x02
	MsgExecReq     byte = 0x03
	MsgExecResp    byte = 0x04
	MsgUploadReq   byte = 0x05
	MsgUploadResp  byte = 0x06
	MsgDownloadReq byte = 0x07
	MsgDownloadResp byte = 0x08
	MsgHealthReq   byte = 0x09
	MsgHealthResp  byte = 0x0A
	MsgError       byte = 0xFF
)

// MaxFrameSize 单帧最大 payload（避免恶意大帧 OOM；1GB 足够大文件）
const MaxFrameSize = 1 << 30

// ---- 请求/响应结构（JSON 序列化） ----

type AuthReq struct {
	Token string `json:"token"`
}

type AuthResp struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type ExecReq struct {
	Cmd     string  `json:"cmd"`
	Timeout float64 `json:"timeout,omitempty"` // 秒，0=默认 3600
	Cwd     string  `json:"cwd,omitempty"`
}

type ExecResp struct {
	Code      int    `json:"code"`
	Cmd       string `json:"cmd,omitempty"`
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	TimedOut  bool   `json:"timed_out"`
	ElapsedMs int64  `json:"elapsed_ms"`
}

type UploadReq struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type UploadResp struct {
	Code  int    `json:"code"`
	Path  string `json:"path,omitempty"`
	Size  int64  `json:"size,omitempty"`
	Error string `json:"error,omitempty"`
}

type DownloadReq struct {
	Path string `json:"path"`
}

type DownloadResp struct {
	Code  int    `json:"code"`
	Path  string `json:"path,omitempty"`
	Size  int64  `json:"size,omitempty"`
	Error string `json:"error,omitempty"`
}

type HealthResp struct {
	OK    bool   `json:"ok"`
	Time  int64  `json:"time"`
	Error string `json:"error,omitempty"`
}

type ErrorResp struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}

// ---- 帧编解码 ----

// Frame 一帧消息
type Frame struct {
	ReqID   uint64
	MsgType byte
	Payload []byte // 不含 reqID/msgType 的纯 payload
}

// Encode 把 Frame 编码为字节切片
func (f *Frame) Encode() []byte {
	totalLen := uint32(8 + 1 + len(f.Payload))
	buf := make([]byte, 4+totalLen)
	binary.BigEndian.PutUint32(buf[0:4], totalLen)
	binary.BigEndian.PutUint64(buf[4:12], f.ReqID)
	buf[12] = f.MsgType
	copy(buf[13:], f.Payload)
	return buf
}

// DecodeFrame 从 reader 读取一帧
func DecodeFrame(r io.Reader) (*Frame, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}
	totalLen := binary.BigEndian.Uint32(lenBuf[:])
	if totalLen < 9 || totalLen > MaxFrameSize {
		return nil, fmt.Errorf("nefsagent: invalid frame length %d", totalLen)
	}
	var restBuf [9]byte
	if _, err := io.ReadFull(r, restBuf[:]); err != nil {
		return nil, err
	}
	rest := make([]byte, totalLen-9)
	if _, err := io.ReadFull(r, rest); err != nil {
		return nil, err
	}
	f := &Frame{
		ReqID:   binary.BigEndian.Uint64(restBuf[0:8]),
		MsgType: restBuf[8],
	}
	if len(rest) > 0 {
		f.Payload = rest
	}
	return f, nil
}

// JSON helpers

func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

func unmarshalJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
