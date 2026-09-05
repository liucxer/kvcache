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
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("dial: %v\n", err)
		return
	}
	defer conn.Close()

	client := pb.NewKVServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prefix := "bench3n:"
	req := &pb.ScanKeysRequest{
		Prefix: prefix,
		Limit:  1000000,
	}
	resp, err := client.ScanKeys(ctx, req)
	if err != nil {
		fmt.Printf("ScanKeys: %v\n", err)
		return
	}
	fmt.Printf("Total keys returned: %d\n", len(resp.Keys))
	if len(resp.Keys) > 0 {
		fmt.Printf("First 5 keys: %v\n", resp.Keys[:5])
		fmt.Printf("Last 5 keys: %v\n", resp.Keys[len(resp.Keys)-5:])
		// Count by worker id
		prefixes := make(map[string]int)
		for _, k := range resp.Keys {
			// format: prefix:wN:seq
			// worker is the middle part
			parts := 0
			var widStart int
			for i, c := range k {
				if c == ':' {
					parts++
					if parts == 1 {
						widStart = i + 1
					}
					if parts == 2 {
						wid := k[widStart:i]
						prefixes[wid]++
						break
					}
				}
			}
		}
		fmt.Printf("Worker id counts (first 20):\n")
		cnt := 0
		for w, c := range prefixes {
			if cnt < 20 {
				fmt.Printf("  w%s: %d keys\n", w, c)
				cnt++
			}
		}
		fmt.Printf("Total unique workers: %d\n", len(prefixes))
	}
}