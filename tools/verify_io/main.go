package main

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "kvcache/proto"
)

func main() {
	addr := "127.0.0.1:33000"
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("dial: %v\n", err)
		return
	}
	defer conn.Close()

	client := pb.NewKeyValueServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	key := "verify_test:key0"
	value := make([]byte, 4*1024*1024) // 4MB
	for i := range value {
		value[i] = byte(i % 256)
	}

	// Set
	setResp, err := client.Set(ctx, &pb.SetRequest{Key: []byte(key), Value: value})
	fmt.Printf("Set: success=%v error=%v err=%v\n", setResp.Success, setResp.Error, err)

	// Get immediately
	getResp, err := client.Get(ctx, &pb.GetRequest{Key: []byte(key)})
	fmt.Printf("Get: found=%v valueLen=%d error=%v err=%v\n", getResp.Found, len(getResp.Value), getResp.Error, err)

	// Get again
	getResp2, err := client.Get(ctx, &pb.GetRequest{Key: []byte(key)})
	fmt.Printf("Get2: found=%v valueLen=%d error=%v err=%v\n", getResp2.Found, len(getResp2.Value), getResp2.Error, err)

	// Try a key that bench wrote
	getResp3, err := client.Get(ctx, &pb.GetRequest{Key: []byte("single2:w0:0")})
	fmt.Printf("Get single2:w0:0: found=%v valueLen=%d error=%v err=%v\n", getResp3.Found, len(getResp3.Value), getResp3.Error, err)

	// Try another key
	getResp4, err := client.Get(ctx, &pb.GetRequest{Key: []byte("single2:w10:100")})
	fmt.Printf("Get single2:w10:100: found=%v valueLen=%d error=%v err=%v\n", getResp4.Found, len(getResp4.Value), getResp4.Error, err)
}