package api

import (
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"kvcache/proto"
)

// SetStream 客户端流式写入：聚合成完整 value 后走原有 service.Set 落盘路径
func (s *GRPCServer) SetStream(stream grpc.ClientStreamingServer[proto.SetChunk, proto.SetResponse]) error {
	var key string
	var buf []byte
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			if key == "" {
				return stream.SendAndClose(&proto.SetResponse{Success: false, Error: "empty key"})
			}
			if err := s.service.Set(stream.Context(), key, buf, 0); err != nil {
				return stream.SendAndClose(&proto.SetResponse{Success: false, Error: err.Error()})
			}
			return stream.SendAndClose(&proto.SetResponse{Success: true})
		}
		if err != nil {
			return err
		}
		if key == "" {
			if len(chunk.Key) == 0 {
				return stream.SendAndClose(&proto.SetResponse{Success: false, Error: "first chunk must carry key"})
			}
			key = string(chunk.Key)
		}
		buf = append(buf, chunk.Data...)
	}
}

// GetStream 服务端流式读取：拿到完整 value 后分帧下发
func (s *GRPCServer) GetStream(req *proto.GetRequest, stream grpc.ServerStreamingServer[proto.GetChunk]) error {
	if len(req.Key) == 0 {
		return status.Error(codes.InvalidArgument, "empty key")
	}

	value, err := s.service.Get(stream.Context(), string(req.Key))
	if err != nil {
		return status.Error(codes.NotFound, err.Error())
	}

	for off := 0; off < len(value); off += proto.StreamChunkSize {
		end := off + proto.StreamChunkSize
		if end > len(value) {
			end = len(value)
		}
		if err := stream.Send(&proto.GetChunk{Data: value[off:end]}); err != nil {
			return err
		}
	}
	return nil
}
