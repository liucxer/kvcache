// nefsproxy CLI：nefs-proxy (proxy.py) 服务的命令行客户端。
//
// 用法示例：
//
//	# 远程执行命令
//	nefsproxy --addr 100.71.128.13:9527 exec "hostname; date"
//
//	# 文件上传/下载
//	nefsproxy --addr 100.71.128.13:9527 upload ./local.txt /root/remote.txt
//	nefsproxy --addr 100.71.128.13:9527 download /root/remote.txt ./local.txt
//
//	# 端口转发
//	nefsproxy --addr 100.71.128.13:9527 proxy-list
//	nefsproxy --addr 100.71.128.13:9527 proxy-add --name test --listen-port 9999 --target-ip 100.71.128.12 --target-port 22
//	nefsproxy --addr 100.71.128.13:9527 proxy-del test
//
//	# 通过 proxy 隧道访问不可达主机（如 128.12 的 22 端口）
//	nefsproxy --addr 100.71.128.13:9527 tunnel --target 100.71.128.12:22
//	# 然后连 proxyHost:listenPort（tunnel 会打印 listenPort）
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"kvcache/nefsproxy"
)

func main() {
	os.Exit(run())
}

func run() int {
	fs := flag.NewFlagSet("nefsproxy", flag.ContinueOnError)
	addr := fs.String("addr", "100.71.128.12:9527", "proxy.py 服务地址 host:port")
	token := fs.String("token", "95279527", "X-Token")
	timeout := fs.Duration("timeout", 120*time.Second, "HTTP 请求超时")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return 2
	}

	args := fs.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "用法: nefsproxy [--addr HOST:PORT] <exec|upload|download|ls|mkdir|delete|ping|proxy-list|proxy-add|proxy-del|tunnel> [args]")
		return 2
	}

	baseURL := *addr
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	client := nefsproxy.NewClient(baseURL, *token, nefsproxy.WithTimeout(*timeout))
	ctx, cancel := context.WithTimeout(context.Background(), *timeout+30*time.Second)
	defer cancel()

	cmd := args[0]
	rest := args[1:]
	var err error

	switch cmd {
	case "ping":
		err = client.Ping(ctx)
		if err == nil {
			fmt.Println("pong")
		}

	case "exec":
		if len(rest) == 0 {
			fmt.Fprintln(os.Stderr, "exec: 缺少命令字符串")
			return 2
		}
		res, e := client.Exec(ctx, strings.Join(rest, " "), 0, "")
		if e != nil {
			err = e
			break
		}
		if res.Stdout != "" {
			fmt.Print(res.Stdout)
			if !strings.HasSuffix(res.Stdout, "\n") {
				fmt.Println()
			}
		}
		if res.Stderr != "" {
			fmt.Fprint(os.Stderr, res.Stderr)
		}
		fmt.Fprintf(os.Stderr, "[exit=%d elapsed=%dms timed_out=%v]\n", res.ExitCode, res.ElapsedMs, res.TimedOut)
		if res.ExitCode != 0 {
			return res.ExitCode
		}

	case "upload":
		if len(rest) != 2 {
			fmt.Fprintln(os.Stderr, "upload <local> <remote>")
			return 2
		}
		var size int64
		size, err = client.UploadFile(ctx, rest[0], rest[1])
		if err == nil {
			fmt.Printf("uploaded %d bytes -> %s\n", size, rest[1])
		}

	case "download":
		if len(rest) != 2 {
			fmt.Fprintln(os.Stderr, "download <remote> <local>")
			return 2
		}
		err = client.DownloadFile(ctx, rest[0], rest[1])
		if err == nil {
			fmt.Printf("downloaded %s -> %s\n", rest[0], rest[1])
		}

	case "ls":
		if len(rest) != 1 {
			fmt.Fprintln(os.Stderr, "ls <remote-path>")
			return 2
		}
		var entries []nefsproxy.Entry
		entries, err = client.Ls(ctx, rest[0])
		if err == nil {
			for _, e := range entries {
				typ := "f"
				if e.Type == "dir" {
					typ = "d"
				}
				fmt.Printf("%s %12d  %s\n", typ, e.Size, e.Name)
			}
		}

	case "mkdir":
		if len(rest) != 1 {
			fmt.Fprintln(os.Stderr, "mkdir <remote-path>")
			return 2
		}
		err = client.Mkdir(ctx, rest[0])
		if err == nil {
			fmt.Println("ok")
		}

	case "delete":
		if len(rest) != 1 {
			fmt.Fprintln(os.Stderr, "delete <remote-path>")
			return 2
		}
		err = client.Delete(ctx, rest[0])
		if err == nil {
			fmt.Println("ok")
		}

	case "proxy-list":
		var rules []nefsproxy.ProxyRule
		rules, err = client.ProxyList(ctx)
		if err == nil {
			for _, r := range rules {
				fmt.Printf("%s  %s:%d -> %s:%d  alive=%v\n", r.Name, r.ListenIP, r.ListenPort, r.TargetIP, r.TargetPort, r.Alive)
			}
		}

	case "proxy-add":
		fs := flag.NewFlagSet("proxy-add", flag.ContinueOnError)
		name := fs.String("name", "", "规则名（空=自动）")
		listenIP := fs.String("listen-ip", "0.0.0.0", "监听 IP")
		listenPort := fs.Int("listen-port", 0, "监听端口")
		targetIP := fs.String("target-ip", "", "目标 IP")
		targetPort := fs.Int("target-port", 0, "目标端口")
		if err := fs.Parse(rest); err != nil {
			return 2
		}
		if *listenPort == 0 || *targetIP == "" || *targetPort == 0 {
			fmt.Fprintln(os.Stderr, "proxy-add 需要 --listen-port --target-ip --target-port")
			return 2
		}
		err = client.ProxyAdd(ctx, nefsproxy.ProxyRule{
			Name: *name, ListenIP: *listenIP, ListenPort: *listenPort,
			TargetIP: *targetIP, TargetPort: *targetPort,
		})
		if err == nil {
			fmt.Println("ok")
		}

	case "proxy-del":
		if len(rest) != 1 {
			fmt.Fprintln(os.Stderr, "proxy-del <name>")
			return 2
		}
		err = client.ProxyDelete(ctx, rest[0])
		if err == nil {
			fmt.Println("ok")
		}

	case "tunnel":
		// 在 proxy 服务端创建转发规则：proxyHost:listenPort -> target
		// 然后本地起端口转发 127.0.0.1:localPort -> proxyHost:listenPort
		// 最后打印本地端口，用户可连 localhost:localPort 访问目标
		fs := flag.NewFlagSet("tunnel", flag.ContinueOnError)
		target := fs.String("target", "", "目标 host:port，如 100.71.128.12:22")
		proxyListenPort := fs.Int("proxy-listen-port", 0, "proxy 端监听端口（0=随机）")
		localPort := fs.Int("local-port", 0, "本地监听端口（0=随机）")
		keep := fs.Bool("keep", false, "保持规则不自动删除（Ctrl+C 退出时仍会删）")
		if err := fs.Parse(rest); err != nil {
			return 2
		}
		if *target == "" {
			fmt.Fprintln(os.Stderr, "tunnel 需要 --target host:port")
			return 2
		}
		targetHost, targetPort, e := splitHostPort(*target)
		if e != nil {
			fmt.Fprintln(os.Stderr, "tunnel target: ", e)
			return 2
		}

		// 解析 proxyHost（--addr 的 host 部分）
		proxyHost, _, _ := net.SplitHostPort(*addr)
		if proxyHost == "" {
			proxyHost = *addr
		}

		// 1. 创建 proxy 转发规则
		rule := nefsproxy.ProxyRule{
			Name:       fmt.Sprintf("go-tunnel-%d", time.Now().UnixNano()),
			ListenIP:   "0.0.0.0",
			ListenPort: *proxyListenPort,
			TargetIP:   targetHost,
			TargetPort: targetPort,
		}
		if e := client.ProxyAdd(ctx, rule); e != nil {
			fmt.Fprintln(os.Stderr, "tunnel add rule:", e)
			return 1
		}
		fmt.Fprintf(os.Stderr, "[tunnel] rule %s created: proxy:%d -> %s:%d\n", rule.Name, rule.ListenPort, targetHost, targetPort)
		if !*keep {
			defer func() {
				delCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = client.ProxyDelete(delCtx, rule.Name)
				fmt.Fprintf(os.Stderr, "[tunnel] rule %s deleted\n", rule.Name)
			}()
		}

		// 2. 本地起 TCP 转发：localPort -> proxyHost:listenPort
		ln, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *localPort))
		if e != nil {
			fmt.Fprintln(os.Stderr, "tunnel local listen:", e)
			return 1
		}
		localPortReal := ln.Addr().(*net.TCPAddr).Port
		fmt.Printf("tunnel ready: connect to 127.0.0.1:%d to reach %s:%d\n", localPortReal, targetHost, targetPort)

		// 阻塞接受连接并转发
		for {
			in, e := ln.Accept()
			if e != nil {
				fmt.Fprintln(os.Stderr, "tunnel accept:", e)
				return 1
			}
			go func(in net.Conn) {
				out, e := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", proxyHost, rule.ListenPort), 10*time.Second)
				if e != nil {
					fmt.Fprintln(os.Stderr, "tunnel dial proxy:", e)
					in.Close()
					return
				}
				pipe(in, out)
			}(in)
		}

	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", cmd)
		return 2
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func splitHostPort(s string) (string, int, error) {
	idx := strings.LastIndex(s, ":")
	if idx < 0 {
		return "", 0, fmt.Errorf("invalid host:port: %s", s)
	}
	host := s[:idx]
	var port int
	_, err := fmt.Sscanf(s[idx+1:], "%d", &port)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port: %s", s[idx+1:])
	}
	return host, port, nil
}

// pipe 双向转发两个连接，任一方向关闭即结束
func pipe(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() {
		_, _ = copyAndClose(a, b)
		done <- struct{}{}
	}()
	go func() {
		_, _ = copyAndClose(b, a)
		done <- struct{}{}
	}()
	<-done
	a.Close()
	b.Close()
	<-done
}

func copyAndClose(dst, src net.Conn) (int64, error) {
	buf := make([]byte, 32*1024)
	n, err := copyBuffer(dst, src, buf)
	// 读端关闭
	if tc, ok := src.(*net.TCPConn); ok {
		_ = tc.CloseRead()
	} else {
		_ = src.Close()
	}
	return n, err
}

func copyBuffer(dst, src net.Conn, buf []byte) (int64, error) {
	var written int64
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = fmt.Errorf("invalid write result")
				}
			}
			written += int64(nw)
			if ew != nil {
				return written, ew
			}
			if nr != nw {
				return written, fmt.Errorf("short write")
			}
		}
		if er != nil {
			if er.Error() == "EOF" {
				return written, nil
			}
			return written, er
		}
	}
}
