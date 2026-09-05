package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "kvcache/proto"
)

func main() {
	addr := "127.0.0.1:33000"
	prefix := "single2:"
	if len(os.Args) > 1 {
		prefix = os.Args[1]
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("dial: %v\n", err)
		return
	}
	defer conn.Close()

	client := pb.NewKeyValueServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &pb.ScanRequest{
		Prefix: []byte(prefix),
	}
	resp, err := client.ScanKeys(ctx, req)
	if err != nil {
		fmt.Printf("ScanKeys: %v\n", err)
		return
	}
	fmt.Printf("prefix=%s total keys: %d\n", prefix, len(resp.Keys))
	if len(resp.Keys) > 0 {
		fmt.Printf("first 5: %v\n", resp.Keys[:min(5, len(resp.Keys))])
		fmt.Printf("last 5:  %v\n", resp.Keys[max(0, len(resp.Keys)-5):])
	}

	// also scan without prefix to see total
	req2 := &pb.ScanRequest{Prefix: []byte("")}
	resp2, err := client.ScanKeys(ctx, req2)
	if err == nil {
		fmt.Printf("total keys (no prefix): %d\n", len(resp2.Keys))
		if len(resp2.Keys) > 0 {
			fmt.Printf("sample keys: %v\n", resp2.Keys[:min(10, len(resp2.Keys))])
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}