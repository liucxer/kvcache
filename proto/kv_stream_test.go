package proto_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"kvcache/proto"
)

// fakeStreamSrv 内存版流式服务，验证手写 stub 的编解码与收发链路
type fakeStreamSrv struct {
	proto.UnimplementedKVStreamServiceServer
	data map[string][]byte
}

func (f *fakeStreamSrv) SetStream(stream grpc.ClientStreamingServer[proto.SetChunk, proto.SetResponse]) error {
	var key string
	var buf []byte
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			f.data[key] = buf
			return stream.SendAndClose(&proto.SetResponse{Success: true})
		}
		if err != nil {
			return err
		}
		if key == "" {
			key = string(chunk.Key)
		}
		buf = append(buf, chunk.Data...)
	}
}

func (f *fakeStreamSrv) GetStream(req *proto.GetRequest, stream grpc.ServerStreamingServer[proto.GetChunk]) error {
	v, ok := f.data[string(req.Key)]
	if !ok {
		return nil
	}
	for off := 0; off < len(v); off += 1 << 20 {
		end := off + 1<<20
		if end > len(v) {
			end = len(v)
		}
		if err := stream.Send(&proto.GetChunk{Data: v[off:end]}); err != nil {
			return err
		}
	}
	return nil
}

func TestStreamRoundTrip(t *testing.T) {
	lis := bufconn.Listen(8 << 20)
	srv := grpc.NewServer()
	proto.RegisterKVStreamServiceServer(srv, &fakeStreamSrv{data: map[string][]byte{}})
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	conn, err := grpc.Dial("bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	cli := proto.NewKVStreamServiceClient(conn)
	ctx := context.Background()

	// 4.5MB value：跨多个 1MB chunk + 一个零头 chunk
	value := make([]byte, (4<<20)+(512<<10))
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}

	ss, err := cli.SetStream(ctx)
	if err != nil {
		t.Fatalf("SetStream: %v", err)
	}
	for off := 0; ; off += 1 << 20 {
		end := off + 1<<20
		if end > len(value) {
			end = len(value)
		}
		chunk := &proto.SetChunk{Data: value[off:end]}
		if off == 0 {
			chunk.Key = []byte("k1")
		}
		if err := ss.Send(chunk); err != nil {
			t.Fatalf("send: %v", err)
		}
		if end == len(value) {
			break
		}
	}
	resp, err := ss.CloseAndRecv()
	if err != nil {
		t.Fatalf("CloseAndRecv: %v", err)
	}
	if !resp.Success {
		t.Fatalf("set rejected: %s", resp.Error)
	}

	gs, err := cli.GetStream(ctx, &proto.GetRequest{Key: []byte("k1")})
	if err != nil {
		t.Fatalf("GetStream: %v", err)
	}
	var got []byte
	for {
		chunk, err := gs.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		got = append(got, chunk.Data...)
	}
	if !bytes.Equal(got, value) {
		t.Fatalf("round trip mismatch: got %d bytes, want %d", len(got), len(value))
	}
}
