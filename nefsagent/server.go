package nefsagent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// Server 是 nefsagent 服务端：同一端口同时支持 RPC 长连接协议与
// Python proxy.py 兼容的 HTTP 接口。
type Server struct {
	addr     string
	token    string
	listener net.Listener

	// worker pool 并发执行命令
	cmdCh chan *execJob

	// HTTP（proxy.py 兼容接口）
	httpLn  *httpConnListener
	httpSrv *http.Server
	proxy   *ProxyManager
}

type execJob struct {
	req    *ExecReq
	respCh chan *ExecResp
}

// ServerOption 配置 Server
type ServerOption func(*Server)

// WithWorkerCount 设置命令执行并发度
func WithWorkerCount(n int) ServerOption {
	return func(s *Server) {
		if n > 0 {
			s.cmdCh = make(chan *execJob, n*4)
			for i := 0; i < n; i++ {
				go s.runCmdWorker()
			}
		}
	}
}

// NewServer 创建服务端
func NewServer(addr, token string, opts ...ServerOption) *Server {
	s := &Server{
		addr:    addr,
		token:   token,
		cmdCh:   make(chan *execJob, 64),
		proxy:   NewProxyManager(),
		httpLn:  newHTTPConnListener(),
	}
	for _, o := range opts {
		o(s)
	}
	// 默认启动 8 个 worker
	if cap(s.cmdCh) == 64 {
		for i := 0; i < 8; i++ {
			go s.runCmdWorker()
		}
	}
	return s
}

// ListenAndServe 启动服务，阻塞。同一端口支持 RPC 长连接与 HTTP。
func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("nefsagent: listen %s: %w", s.addr, err)
	}
	s.listener = ln
	fmt.Printf("[nefsagent] listening on %s (token set=%v, http proxy.py compatible)\n", s.addr, s.token != "")

	// HTTP 服务：连接由 accept 循环嗅探后推入
	s.httpSrv = &http.Server{
		Handler:           (&AgentHTTP{Token: s.token, Srv: s, Proxy: s.proxy}).Handler(),
		ReadHeaderTimeout: 30 * time.Second,
	}
	go func() {
		if err := s.httpSrv.Serve(s.httpLn); err != nil && err != http.ErrServerClosed {
			fmt.Printf("[nefsagent] http serve: %v\n", err)
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return fmt.Errorf("nefsagent: accept: %w", err)
		}
		bc, isHTTP := sniffConn(conn)
		if bc == nil {
			continue
		}
		if isHTTP {
			s.httpLn.ch <- bc
		} else {
			go s.handleConn(bc)
		}
	}
}

// Close 关闭服务
func (s *Server) Close() error {
	if s.httpSrv != nil {
		_ = s.httpSrv.Close()
	}
	if s.httpLn != nil {
		_ = s.httpLn.Close()
	}
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// Exec 通过 worker 池执行命令（HTTP 路径复用同一并发池）
func (s *Server) Exec(req *ExecReq) *ExecResp {
	job := &execJob{req: req, respCh: make(chan *ExecResp, 1)}
	s.cmdCh <- job
	return <-job.respCh
}

// handleConn 处理单条连接：鉴权 + 多路复用
func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()

	// 1. 鉴权：第一帧必须是 Auth
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	frame, err := DecodeFrame(conn)
	if err != nil {
		return
	}
	if frame.MsgType != MsgAuthReq {
		resp := &AuthResp{OK: false, Error: "first frame must be Auth"}
		s.writeFrame(conn, 0, MsgAuthResp, resp)
		return
	}
	var authReq AuthReq
	if err := json.Unmarshal(frame.Payload, &authReq); err != nil {
		return
	}
	if s.token != "" && authReq.Token != s.token {
		resp := &AuthResp{OK: false, Error: "invalid token"}
		s.writeFrame(conn, frame.ReqID, MsgAuthResp, resp)
		return
	}
	s.writeFrame(conn, frame.ReqID, MsgAuthResp, &AuthResp{OK: true})
	conn.SetReadDeadline(time.Time{}) // 取消超时

	// 2. 多路复用：reader goroutine 读帧路由，writer goroutine 串行写
	writeMu := &sync.Mutex{}
	writeFrame := func(f *Frame) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_, err := conn.Write(f.Encode())
		return err
	}

	// reader：单 goroutine 独占读 TCP，主线程不抢读
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			frame, err := DecodeFrame(conn)
			if err != nil {
				return
			}
			switch frame.MsgType {
			case MsgHealthReq:
				s.sendResp(writeFrame, frame.ReqID, MsgHealthResp, &HealthResp{OK: true, Time: time.Now().Unix()})
			case MsgExecReq:
				var req ExecReq
				if err := json.Unmarshal(frame.Payload, &req); err != nil {
					s.sendError(writeFrame, frame.ReqID, "bad exec req: "+err.Error())
					continue
				}
				job := &execJob{req: &req, respCh: make(chan *ExecResp, 1)}
				s.cmdCh <- job
				// 异步等待结果再写回（多路复用关键：reader 不阻塞在命令执行上）
				go func(reqID uint64) {
					resp := <-job.respCh
					s.sendResp(writeFrame, reqID, MsgExecResp, resp)
				}(frame.ReqID)
			case MsgUploadReq:
				s.handleUpload(conn, writeFrame, frame)
			case MsgDownloadReq:
				s.handleDownload(conn, writeFrame, frame)
			default:
				s.sendError(writeFrame, frame.ReqID, "unknown msg type")
			}
		}
	}()

	// 等 reader 退出（连接断开或读错误）后 handleConn 返回，defer 关闭连接
	<-done
	_ = remote
}

// sendResp 序列化 v 为 JSON 并写回
func (s *Server) sendResp(writeFrame func(*Frame) error, reqID uint64, msgType byte, v any) {
	payload, err := json.Marshal(v)
	if err != nil {
		s.sendError(writeFrame, reqID, "marshal: "+err.Error())
		return
	}
	writeFrame(&Frame{ReqID: reqID, MsgType: msgType, Payload: payload})
}

func (s *Server) sendError(writeFrame func(*Frame) error, reqID uint64, msg string) {
	payload, _ := json.Marshal(&ErrorResp{Code: 1, Error: msg})
	writeFrame(&Frame{ReqID: reqID, MsgType: MsgError, Payload: payload})
}

// writeFrame 直接写 JSON 响应（连接级）
func (s *Server) writeFrame(conn net.Conn, reqID uint64, msgType byte, v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = conn.Write((&Frame{ReqID: reqID, MsgType: msgType, Payload: payload}).Encode())
	return err
}

// runCmdWorker 从 cmdCh 取任务执行
func (s *Server) runCmdWorker() {
	for job := range s.cmdCh {
		job.respCh <- s.runExec(job.req)
	}
}

// runExec 执行命令（start_new_session + 超时 killpg）
func (s *Server) runExec(req *ExecReq) *ExecResp {
	start := time.Now()
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 3600
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout*float64(time.Second)))
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", req.Cmd)
	cmd.Dir = req.Cwd
	configureCmd(cmd)

	var stdout, stderr []byte
	var err error
	if stdout, err = cmd.Output(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = exitErr.Stderr
		}
	}

	resp := &ExecResp{
		Code:      0,
		ExitCode:  cmd.ProcessState.ExitCode(),
		Stdout:    string(stdout),
		Stderr:    string(stderr),
		ElapsedMs: time.Since(start).Milliseconds(),
	}
	if ctx.Err() == context.DeadlineExceeded {
		resp.TimedOut = true
		resp.Code = 1
		// 整组 kill（平台相关）
		killProcessGroup(cmd)
	}
	if cmd.ProcessState == nil {
		resp.ExitCode = -1
	}
	return resp
}

// handleUpload 处理上传：payload = JSON(UploadReq) + raw bytes
func (s *Server) handleUpload(conn net.Conn, writeFrame func(*Frame) error, frame *Frame) {
	// payload 前半段是 JSON，后半段是 raw bytes
	// 找第一个 '}' 作为 JSON 结束（UploadReq 结构简单无嵌套对象）
	end := -1
	for i := 0; i < len(frame.Payload); i++ {
		if frame.Payload[i] == '}' {
			end = i + 1
			break
		}
	}
	if end < 0 {
		s.sendError(writeFrame, frame.ReqID, "upload: bad json header")
		return
	}
	var req UploadReq
	if err := json.Unmarshal(frame.Payload[:end], &req); err != nil {
		s.sendError(writeFrame, frame.ReqID, "upload: "+err.Error())
		return
	}
	raw := frame.Payload[end:]

	// 写入文件
	if dir := filepath.Dir(req.Path); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}
	if err := os.WriteFile(req.Path, raw, 0644); err != nil {
		s.sendError(writeFrame, frame.ReqID, "upload: "+err.Error())
		return
	}
	s.sendResp(writeFrame, frame.ReqID, MsgUploadResp, &UploadResp{
		Code: 0, Path: req.Path, Size: int64(len(raw)),
	})
}

// handleDownload 处理下载
func (s *Server) handleDownload(conn net.Conn, writeFrame func(*Frame) error, frame *Frame) {
	var req DownloadReq
	if err := json.Unmarshal(frame.Payload, &req); err != nil {
		s.sendError(writeFrame, frame.ReqID, "download: "+err.Error())
		return
	}
	info, err := os.Stat(req.Path)
	if err != nil {
		s.sendError(writeFrame, frame.ReqID, "download: "+err.Error())
		return
	}
	if info.IsDir() {
		s.sendError(writeFrame, frame.ReqID, "download: is dir")
		return
	}
	data, err := os.ReadFile(req.Path)
	if err != nil {
		s.sendError(writeFrame, frame.ReqID, "download: "+err.Error())
		return
	}
	resp := &DownloadResp{Code: 0, Path: req.Path, Size: info.Size()}
	header, _ := json.Marshal(resp)
	// payload = JSON header + raw bytes
	payload := append(header, data...)
	writeFrame(&Frame{ReqID: frame.ReqID, MsgType: MsgDownloadResp, Payload: payload})
	_ = io.EOF
}


