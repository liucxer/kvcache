// 流式传输扩展：本机没有 protoc，本文件按 protoc-gen-go-grpc v1.6.1 的
// 生成模式手写。消息类型走 legacy V1 桥（struct tag + ProtoMessage），
// grpc v1.64+ 的 proto codec 会通过 protoadapt.MessageV2Of 自动适配。
// 与原 kv.proto 里的 unary RPC 并存，互不影响。
package proto

import (
	context "context"
	fmt "fmt"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// SetChunk 流式写入的一个分帧。key 只在首帧携带，后续帧只带 data。
type SetChunk struct {
	Key  []byte `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Data []byte `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *SetChunk) Reset()         { *x = SetChunk{} }
func (x *SetChunk) String() string { return fmt.Sprintf("SetChunk{key:%q data:%dB}", x.Key, len(x.Data)) }
func (*SetChunk) ProtoMessage()    {}

// GetChunk 流式读取的一个分帧，流结束（EOF）即 value 结束。
type GetChunk struct {
	Data []byte `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *GetChunk) Reset()         { *x = GetChunk{} }
func (x *GetChunk) String() string { return fmt.Sprintf("GetChunk{data:%dB}", len(x.Data)) }
func (*GetChunk) ProtoMessage()    {}

const (
	KVStreamService_SetStream_FullMethodName = "/kv.KVStreamService/SetStream"
	KVStreamService_GetStream_FullMethodName = "/kv.KVStreamService/GetStream"
)

// StreamChunkSize 流式传输的分帧大小：1MB。
// 大 value 分帧后，收发两端的 marshal/IO 能流水线重叠，
// 比 unary 整包传输的端到端延迟低。
const StreamChunkSize = 1 << 20

// KVStreamServiceClient 流式客户端
type KVStreamServiceClient interface {
	// 客户端流：分帧发送 value，CloseAndRecv 拿到 SetResponse
	SetStream(ctx context.Context, opts ...grpc.CallOption) (grpc.ClientStreamingClient[SetChunk, SetResponse], error)
	// 服务端流：发 GetRequest，分帧收 value
	GetStream(ctx context.Context, in *GetRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[GetChunk], error)
}

type kvStreamServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewKVStreamServiceClient(cc grpc.ClientConnInterface) KVStreamServiceClient {
	return &kvStreamServiceClient{cc}
}

func (c *kvStreamServiceClient) SetStream(ctx context.Context, opts ...grpc.CallOption) (grpc.ClientStreamingClient[SetChunk, SetResponse], error) {
	stream, err := c.cc.NewStream(ctx, &KVStreamService_ServiceDesc.Streams[0], KVStreamService_SetStream_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	return &grpc.GenericClientStream[SetChunk, SetResponse]{ClientStream: stream}, nil
}

func (c *kvStreamServiceClient) GetStream(ctx context.Context, in *GetRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[GetChunk], error) {
	stream, err := c.cc.NewStream(ctx, &KVStreamService_ServiceDesc.Streams[1], KVStreamService_GetStream_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[GetRequest, GetChunk]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// KVStreamServiceServer 流式服务端
type KVStreamServiceServer interface {
	SetStream(grpc.ClientStreamingServer[SetChunk, SetResponse]) error
	GetStream(*GetRequest, grpc.ServerStreamingServer[GetChunk]) error
}

// UnimplementedKVStreamServiceServer 前向兼容占位
type UnimplementedKVStreamServiceServer struct{}

func (UnimplementedKVStreamServiceServer) SetStream(grpc.ClientStreamingServer[SetChunk, SetResponse]) error {
	return status.Error(codes.Unimplemented, "method SetStream not implemented")
}
func (UnimplementedKVStreamServiceServer) GetStream(*GetRequest, grpc.ServerStreamingServer[GetChunk]) error {
	return status.Error(codes.Unimplemented, "method GetStream not implemented")
}

func RegisterKVStreamServiceServer(s grpc.ServiceRegistrar, srv KVStreamServiceServer) {
	s.RegisterService(&KVStreamService_ServiceDesc, srv)
}

func _KVStreamService_SetStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(KVStreamServiceServer).SetStream(&grpc.GenericServerStream[SetChunk, SetResponse]{ServerStream: stream})
}

func _KVStreamService_GetStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(GetRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(KVStreamServiceServer).GetStream(m, &grpc.GenericServerStream[GetRequest, GetChunk]{ServerStream: stream})
}

// KVStreamService_ServiceDesc grpc.ServiceDesc，服务名与 unary 的
// kv.KeyValueService 不同，挂在同一个 grpc.Server 上不冲突。
var KVStreamService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "kv.KVStreamService",
	HandlerType: (*KVStreamServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "SetStream",
			Handler:       _KVStreamService_SetStream_Handler,
			ClientStreams: true,
		},
		{
			StreamName:    "GetStream",
			Handler:       _KVStreamService_GetStream_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "kv.proto",
}
