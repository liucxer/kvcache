// 零拷贝编解码器：绕过 proto marshal/unmarshal，大 value 以原始字节帧传输。
//
// 背景：4MB value 走 proto 编解码时，客户端 marshal、服务端 unmarshal 各产生
// 一次全量拷贝，pprof 显示 memmove 占 CPU 约 50%，而 146 实测整机并行拷贝
// 带宽只有 ~26GB/s——每省一次拷贝都直接变成吞吐。
//
// 帧格式（大端）：
//   SetRequest:  [4B keyLen][key][value]
//   GetRequest:  [4B keyLen][key]
//   SetResponse: [1B success][4B errLen][err]
//   GetResponse: [1B found][4B errLen][err][value]
//
// Marshal 对 Key/Value 做别名引用（mem.NewBuffer(pool=nil) 不拷贝）：
// 客户端调用方在 unary RPC 返回前不得修改传入的 Key/Value（unary 语义下
// 服务端必定已收完整请求才响应，因此返回后即可复用缓冲区）。
// 服务端 GetResponse.Value 来自 service.Get 新分配的缓冲区，无并发修改。
//
// 使用方式：客户端逐调用 opt-in（grpc.ForceCodecV2(proto.RawCodecV2)），
// 未注册该 codec 的对端不受影响（按 content-subtype 选择，默认 proto 路径不变）。
package proto

import (
	"encoding/binary"
	"fmt"

	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/mem"
)

// RawCodecV2 供客户端 grpc.ForceCodecV2(proto.RawCodecV2) 逐调用启用
var RawCodecV2 encoding.CodecV2 = rawCodec{}

func init() {
	encoding.RegisterCodecV2(rawCodec{})
}

type rawCodec struct{}

func (rawCodec) Name() string { return "rawbytes" }

func (rawCodec) Marshal(v any) (mem.BufferSlice, error) {
	switch m := v.(type) {
	case *SetRequest:
		hdr := make([]byte, 4)
		binary.BigEndian.PutUint32(hdr, uint32(len(m.Key)))
		return mem.BufferSlice{
			mem.NewBuffer(&hdr, nil),
			mem.NewBuffer(&m.Key, nil),
			mem.NewBuffer(&m.Value, nil),
		}, nil
	case *GetRequest:
		hdr := make([]byte, 4)
		binary.BigEndian.PutUint32(hdr, uint32(len(m.Key)))
		return mem.BufferSlice{
			mem.NewBuffer(&hdr, nil),
			mem.NewBuffer(&m.Key, nil),
		}, nil
	case *SetResponse:
		eb := []byte(m.Error)
		out := make([]byte, 5+len(eb))
		if m.Success {
			out[0] = 1
		}
		binary.BigEndian.PutUint32(out[1:5], uint32(len(eb)))
		copy(out[5:], eb)
		return mem.BufferSlice{mem.NewBuffer(&out, nil)}, nil
	case *GetResponse:
		eb := []byte(m.Error)
		hdr := make([]byte, 5+len(eb))
		if m.Found {
			hdr[0] = 1
		}
		binary.BigEndian.PutUint32(hdr[1:5], uint32(len(eb)))
		copy(hdr[5:], eb)
		return mem.BufferSlice{
			mem.NewBuffer(&hdr, nil),
			mem.NewBuffer(&m.Value, nil),
		}, nil
	}
	return nil, fmt.Errorf("rawCodec: unsupported type %T", v)
}

func (rawCodec) Unmarshal(data mem.BufferSlice, v any) error {
	// 一次性物化为连续缓冲（全程仅此一次拷贝），字段切片共享底层数组
	all := data.Materialize()

	switch m := v.(type) {
	case *SetRequest:
		if len(all) < 4 {
			return fmt.Errorf("rawCodec: short SetRequest frame")
		}
		kl := int(binary.BigEndian.Uint32(all[:4]))
		if len(all) < 4+kl {
			return fmt.Errorf("rawCodec: truncated SetRequest key")
		}
		m.Key = all[4 : 4+kl]
		m.Value = all[4+kl:]
		return nil
	case *GetRequest:
		if len(all) < 4 {
			return fmt.Errorf("rawCodec: short GetRequest frame")
		}
		kl := int(binary.BigEndian.Uint32(all[:4]))
		if len(all) < 4+kl {
			return fmt.Errorf("rawCodec: truncated GetRequest key")
		}
		m.Key = all[4 : 4+kl]
		return nil
	case *SetResponse:
		if len(all) < 5 {
			return fmt.Errorf("rawCodec: short SetResponse frame")
		}
		m.Success = all[0] == 1
		el := int(binary.BigEndian.Uint32(all[1:5]))
		if len(all) < 5+el {
			return fmt.Errorf("rawCodec: truncated SetResponse error")
		}
		m.Error = string(all[5 : 5+el])
		return nil
	case *GetResponse:
		if len(all) < 5 {
			return fmt.Errorf("rawCodec: short GetResponse frame")
		}
		m.Found = all[0] == 1
		el := int(binary.BigEndian.Uint32(all[1:5]))
		if len(all) < 5+el {
			return fmt.Errorf("rawCodec: truncated GetResponse error")
		}
		m.Error = string(all[5 : 5+el])
		m.Value = all[5+el:]
		return nil
	}
	return fmt.Errorf("rawCodec: unsupported type %T", v)
}
