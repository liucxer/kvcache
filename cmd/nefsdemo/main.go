// nefsdemo: Windows 常驻统一进程（单进程，不每次启动 CLI）
//
// 同时提供两套入口，共用同一个到远端 128.12:9528 的长连接池：
//  1. Web UI    http://127.0.0.1:8080   浏览器操作
//  2. 本地 IPC  127.0.0.1:19528         nefsagent-cli 的 exec/ping 快速通道
//
// 管理：
//  nefsagent-cli daemon start|stop|status|install|uninstall
//
// 用法：
//
//	nefsdemo [--addr 100.71.128.12:9528] [--token nefsagent]
//	        [--listen 127.0.0.1:8080] [--ipc 127.0.0.1:19528] [--no-ipc]
package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"kvcache/nefsagent"
)

//go:embed index.html
var staticFS embed.FS

func main() {
	addr := flag.String("addr", "100.71.128.12:9528", "远端 nefsagent 地址")
	token := flag.String("token", "nefsagent", "鉴权 token")
	listen := flag.String("listen", "127.0.0.1:8080", "Web UI 监听地址")
	ipc := flag.String("ipc", nefsagent.IPCAddr, "本地 IPC 监听地址")
	noIPC := flag.Bool("no-ipc", false, "禁用本地 IPC（仅 Web UI）")
	flag.Parse()

	c, err := nefsagent.NewClient(*addr, *token, nefsagent.WithPoolSize(4))
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect %s: %v\n", *addr, err)
		os.Exit(1)
	}
	defer c.Close()

	// PID 文件（供 nefsagent-cli daemon 管理）
	if err := nefsagent.WritePID(); err != nil {
		fmt.Fprintf(os.Stderr, "write pid: %v\n", err)
	}
	defer nefsagent.RemovePID()

	fmt.Printf("[nefsdemo] connected %s (token=%s)\n", *addr, *token)

	// HTTP 服务
	mux := newMux(c, *addr, *token, *listen)
	srv := &http.Server{Addr: *listen, Handler: mux}
	go func() {
		fmt.Printf("[nefsdemo] web ui: http://%s\n", *listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "listen %s: %v\n", *listen, err)
			os.Exit(1)
		}
	}()

	// IPC 服务（CLI 快速通道）
	if !*noIPC {
		go func() {
			ln, err := net.Listen("tcp", *ipc)
			if err != nil {
				fmt.Fprintf(os.Stderr, "listen ipc %s: %v\n", *ipc, err)
				os.Exit(1)
			}
			fmt.Printf("[nefsdemo] ipc: %s (nefsagent-cli exec/ping)\n", *ipc)
			for {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				go handleIPCConn(conn, c)
			}
		}()
	}

	// 常驻（demo 不空闲退出）
	select {}
}

// newMux 构建 HTTP handler
func newMux(c *nefsagent.Client, addr, token, listen string) *http.ServeMux {
	idx, _ := staticFS.ReadFile("index.html")
	tmpl := template.Must(template.New("index").Parse(string(idx)))

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		tmpl.Execute(w, struct {
			Addr, Token, Listen string
		}{addr, token, listen})
	})

	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		err := c.Health(r.Context())
		writeJSON(w, map[string]any{
			"ok":      err == nil,
			"error":   errStr(err),
			"latency": time.Since(start).Milliseconds(),
		})
	})

	mux.HandleFunc("/api/exec", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		var req struct {
			Cmd     string  `json:"cmd"`
			Cwd     string  `json:"cwd"`
			Timeout float64 `json:"timeout"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "bad json: " + err.Error()})
			return
		}

		// context 超时 = 命令超时 + 缓冲；无显式超时给 2h 兜底（服务端默认 1h）
		to := 2 * time.Hour
		if req.Timeout > 0 {
			to = time.Duration(req.Timeout*float64(time.Second)) + 30*time.Second
		}
		ctx, cancel := context.WithTimeout(r.Context(), to)
		defer cancel()

		start := time.Now()
		resp, err := c.Exec(ctx, req.Cmd, time.Duration(req.Timeout*float64(time.Second)), req.Cwd)
		latency := time.Since(start)

		if err != nil {
			writeJSON(w, map[string]any{
				"ok": false, "error": err.Error(), "latency_ms": latency.Milliseconds(),
			})
			return
		}
		writeJSON(w, map[string]any{
			"ok":         true,
			"stdout":     resp.Stdout,
			"stderr":     resp.Stderr,
			"exit_code":  resp.ExitCode,
			"timed_out":  resp.TimedOut,
			"latency_ms": latency.Milliseconds(),
		})
	})
	return mux
}

// handleIPCConn 处理 nefsagent-cli 的本地 IPC 请求
func handleIPCConn(conn net.Conn, c *nefsagent.Client) {
	defer conn.Close()
	// 读请求用短 deadline（防慢客户端占用）
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	// 读请求长度
	var lenBuf [4]byte
	if _, err := io.ReadFull(conn, lenBuf[:]); err != nil {
		return
	}
	reqLen := int(lenBuf[0])<<24 | int(lenBuf[1])<<16 | int(lenBuf[2])<<8 | int(lenBuf[3])
	reqData := make([]byte, reqLen)
	if _, err := io.ReadFull(conn, reqData); err != nil {
		return
	}
	var req nefsagent.IPCRequest
	if err := json.Unmarshal(reqData, &req); err != nil {
		sendIPCResp(conn, &nefsagent.IPCResponse{OK: false, Error: "bad request: " + err.Error()})
		return
	}

	// 处理阶段 deadline = 命令超时 + 缓冲；无显式超时给 2h（服务端默认 1h）
	deadline := 2 * time.Hour
	if req.Timeout > 0 {
		deadline = time.Duration(req.Timeout*float64(time.Second)) + 30*time.Second
	}
	conn.SetDeadline(time.Now().Add(deadline))

	ctx := context.Background()
	switch req.Op {
	case "exec":
		var to time.Duration
		if req.Timeout > 0 {
			to = time.Duration(req.Timeout * float64(time.Second))
		}
		resp, err := c.Exec(ctx, req.Cmd, to, req.Cwd)
		if err != nil {
			sendIPCResp(conn, &nefsagent.IPCResponse{OK: false, Error: err.Error()})
			return
		}
		sendIPCResp(conn, &nefsagent.IPCResponse{
			OK: true, ExitCode: resp.ExitCode, TimedOut: resp.TimedOut,
			Stdout: resp.Stdout, Stderr: resp.Stderr,
		})
	case "ping":
		if err := c.Health(ctx); err != nil {
			sendIPCResp(conn, &nefsagent.IPCResponse{OK: false, Error: err.Error()})
			return
		}
		sendIPCResp(conn, &nefsagent.IPCResponse{OK: true})
	default:
		sendIPCResp(conn, &nefsagent.IPCResponse{OK: false, Error: "unknown op: " + req.Op})
	}
}

func sendIPCResp(conn net.Conn, resp *nefsagent.IPCResponse) {
	data, _ := json.Marshal(resp)
	header := make([]byte, 4)
	header[0] = byte(len(data) >> 24)
	header[1] = byte(len(data) >> 16)
	header[2] = byte(len(data) >> 8)
	header[3] = byte(len(data))
	conn.Write(header)
	conn.Write(data)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
