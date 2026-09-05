// nefsagent: 高性能长连接 RPC 服务端
package main

import (
	"flag"
	"fmt"
	"os"

	"kvcache/nefsagent"
)

func main() {
	addr := flag.String("addr", ":9528", "监听地址")
	token := flag.String("token", "nefsagent", "鉴权 token")
	workers := flag.Int("workers", 8, "命令执行 worker 数")
	flag.Parse()

	srv := nefsagent.NewServer(*addr, *token, nefsagent.WithWorkerCount(*workers))
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
