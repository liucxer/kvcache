// uploadbin: base64 分块上传二进制到 128.12（绕开 multipart 问题）
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"kvcache/nefsproxy"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: uploadbin <local> <remote>")
		return
	}
	local, remote := os.Args[1], os.Args[2]
	data, err := os.ReadFile(local)
	if err != nil {
		fmt.Println("read err:", err)
		os.Exit(1)
	}
	fmt.Printf("local file: %d bytes\n", len(data))

	c := nefsproxy.NewClient("http://100.71.128.12:9527", "95279527", nefsproxy.WithTimeout(30*time.Second))
	ctx := context.Background()

	// 先清掉远程文件
	c.Exec(ctx, fmt.Sprintf("rm -f %s; touch %s", remote, remote), 0, "")

	// 分块：每块 32KB 原始 → ~43KB base64，避开 ARG_MAX 限制
	chunkSize := 32 * 1024
	totalChunks := (len(data) + chunkSize - 1) / chunkSize
	fmt.Printf("uploading in %d chunks...\n", totalChunks)

	for i := 0; i < totalChunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}
		b64 := base64.StdEncoding.EncodeToString(data[start:end])
		// 单引号包裹 base64（无特殊字符），管道解码追加
		cmd := fmt.Sprintf("echo -n '%s' | base64 -d >> %s", b64, remote)
		resp, err := c.Exec(ctx, cmd, 30, "")
		if err != nil {
			fmt.Printf("chunk %d/%d err: %v\n", i+1, totalChunks, err)
			os.Exit(1)
		}
		if resp.ExitCode != 0 {
			fmt.Printf("chunk %d/%d exit=%d stderr=%s\n", i+1, totalChunks, resp.ExitCode, resp.Stderr)
			os.Exit(1)
		}
		fmt.Printf("  chunk %d/%d done (%d bytes)\n", i+1, totalChunks, end-start)
	}

	// 验证大小
	resp, err := c.Exec(ctx, fmt.Sprintf("ls -la %s; file %s", remote, remote), 0, "")
	if err != nil {
		fmt.Println("verify err:", err)
		os.Exit(1)
	}
	fmt.Println("verify:", resp.Stdout)
}
