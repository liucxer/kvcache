package nefsagent

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// ---- 协议嗅探：同一端口同时支持 RPC 与 HTTP ----
//
// RPC 帧首字节是 totalLen(4B) 的高字节，最大 0x40（MaxFrameSize=1GB）；
// HTTP 请求首字节是方法首字母（A-Z，0x41-0x5A）。据此区分。

// sniffConn 读取首字节判断协议，返回带缓冲的连接（已含被嗅探的字节）
func sniffConn(conn net.Conn) (net.Conn, bool) {
	br := bufio.NewReader(conn)
	b, err := br.Peek(1)
	if err != nil {
		conn.Close()
		return nil, false
	}
	bc := &bufferedConn{Conn: conn, r: br}
	return bc, b[0] >= 0x41 && b[0] <= 0x5A
}

// bufferedConn 包装连接，Read 优先从缓冲读（replay 被嗅探的字节）
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.r.Read(p) }

// httpConnListener 把外部推入的 HTTP 连接交给 http.Server
type httpConnListener struct {
	ch    chan net.Conn
	close chan struct{}
}

func newHTTPConnListener() *httpConnListener {
	return &httpConnListener{ch: make(chan net.Conn), close: make(chan struct{})}
}

func (l *httpConnListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.ch:
		return c, nil
	case <-l.close:
		return nil, net.ErrClosed
	}
}

func (l *httpConnListener) Close() error {
	select {
	case <-l.close:
	default:
		close(l.close)
	}
	return nil
}

func (l *httpConnListener) Addr() net.Addr { return dummyAddr("nefsagent-http") }

type dummyAddr string

func (a dummyAddr) Network() string { return "tcp" }
func (a dummyAddr) String() string  { return string(a) }

// ---- HTTP 接口（与 Python proxy.py 完全一致） ----

// AgentHTTP 提供 proxy.py 兼容的 HTTP 接口。
// 除 /ping 外均需 X-Token 头鉴权。
type AgentHTTP struct {
	Token string
	Srv   *Server
	Proxy *ProxyManager
}

// Handler 构建路由
func (h *AgentHTTP) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", h.handlePing) // 无需 token
	mux.HandleFunc("/exec", h.requireAuth(h.handleExec))
	mux.HandleFunc("/upload", h.requireAuth(h.handleUpload))
	mux.HandleFunc("/download", h.requireAuth(h.handleDownload))
	mux.HandleFunc("/ls", h.requireAuth(h.handleLs))
	mux.HandleFunc("/mkdir", h.requireAuth(h.handleMkdir))
	mux.HandleFunc("/delete", h.requireAuth(h.handleDelete))
	mux.HandleFunc("/proxy", h.requireAuth(h.handleProxy))
	mux.HandleFunc("/proxy/add", h.requireAuth(h.handleProxyAdd))
	mux.HandleFunc("/proxy/delete", h.requireAuth(h.handleProxyDelete))
	return mux
}

func (h *AgentHTTP) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.Token != "" && r.Header.Get("X-Token") != h.Token {
			writeErr(w, http.StatusUnauthorized, "invalid token")
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"code": status, "error": msg})
}

// GET /ping → {"status":"ok","time":<unix>}
func (h *AgentHTTP) handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().Unix()})
}

// POST /exec（body {"cmd","timeout","cwd"}）或 GET /exec?cmd=...&timeout=...&cwd=...
// → {"code","cmd","exit_code","stdout","stderr","timed_out","elapsed_ms"}
func (h *AgentHTTP) handleExec(w http.ResponseWriter, r *http.Request) {
	var req ExecReq
	switch r.Method {
	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "exec: "+err.Error())
			return
		}
		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				writeErr(w, http.StatusBadRequest, "exec: bad json: "+err.Error())
				return
			}
		}
	case http.MethodGet:
		q := r.URL.Query()
		req.Cmd = q.Get("cmd")
		req.Cwd = q.Get("cwd")
		if t := q.Get("timeout"); t != "" {
			req.Timeout, _ = strconv.ParseFloat(t, 64)
		}
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if req.Cmd == "" {
		writeErr(w, http.StatusBadRequest, "exec: cmd is empty")
		return
	}
	resp := h.Srv.Exec(&req)
	resp.Cmd = req.Cmd
	writeJSON(w, http.StatusOK, resp)
}

// PUT/POST /upload?path=<urlencoded>，body 为文件原始字节
// → {"code","path","size"}
func (h *AgentHTTP) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		writeErr(w, http.StatusBadRequest, "upload: path is required")
		return
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			writeErr(w, http.StatusInternalServerError, "upload: "+err.Error())
			return
		}
	}
	f, err := os.Create(path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "upload: "+err.Error())
		return
	}
	defer f.Close()
	size, err := io.Copy(f, r.Body)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "upload: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "path": path, "size": size})
}

// GET /download?path=<urlencoded> → 原始文件字节
func (h *AgentHTTP) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		writeErr(w, http.StatusBadRequest, "download: path is required")
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "download: "+err.Error())
		return
	}
	if info.IsDir() {
		writeErr(w, http.StatusInternalServerError, "download: is a directory")
		return
	}
	f, err := os.Open(path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "download: "+err.Error())
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	_, _ = io.Copy(w, f)
}

// GET /ls?path=<urlencoded> → {"code","path","entries":[{"name","type","size","mtime"}]}
func (h *AgentHTTP) handleLs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "."
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "ls: "+err.Error())
		return
	}
	type entry struct {
		Name  string `json:"name"`
		Type  string `json:"type"` // "dir" | "file"
		Size  int64  `json:"size"`
		Mtime int64  `json:"mtime"`
	}
	list := make([]entry, 0, len(entries))
	for _, e := range entries {
		info, ierr := e.Info()
		item := entry{Name: e.Name(), Type: "file"}
		if ierr == nil {
			if info.IsDir() {
				item.Type = "dir"
			}
			item.Size = info.Size()
			item.Mtime = info.ModTime().Unix()
		}
		list = append(list, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "path": path, "entries": list})
}

// POST /mkdir?path=<urlencoded>（递归建目录）
func (h *AgentHTTP) handleMkdir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		writeErr(w, http.StatusBadRequest, "mkdir: path is required")
		return
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		writeErr(w, http.StatusInternalServerError, "mkdir: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0})
}

// POST /delete?path=<urlencoded>（文件或目录）
func (h *AgentHTTP) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		writeErr(w, http.StatusBadRequest, "delete: path is required")
		return
	}
	if err := os.RemoveAll(path); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0})
}

// GET /proxy → {"code","rules":[...]}；DELETE /proxy?name=... 删除规则
func (h *AgentHTTP) handleProxy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rules := h.Proxy.List()
		if rules == nil {
			rules = []ProxyRule{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "rules": rules})
	case http.MethodDelete:
		name := r.URL.Query().Get("name")
		if name == "" {
			writeErr(w, http.StatusBadRequest, "proxy: name is required")
			return
		}
		if err := h.Proxy.Remove(name); err != nil {
			writeErr(w, http.StatusInternalServerError, "proxy: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "name": name})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// POST /proxy/add，body {"name","listen_ip","listen_port","target_ip","target_port","backlog"}
func (h *AgentHTTP) handleProxyAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var rule ProxyRule
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "proxy/add: "+err.Error())
		return
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &rule); err != nil {
			writeErr(w, http.StatusBadRequest, "proxy/add: bad json: "+err.Error())
			return
		}
	}
	if err := h.Proxy.Add(&rule); err != nil {
		writeErr(w, http.StatusInternalServerError, "proxy/add: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "rule": rule})
}

// POST /proxy/delete，body {"name"}
func (h *AgentHTTP) handleProxyDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "proxy/delete: "+err.Error())
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "proxy/delete: bad json: "+err.Error())
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "proxy/delete: name is required")
		return
	}
	if err := h.Proxy.Remove(req.Name); err != nil {
		writeErr(w, http.StatusInternalServerError, "proxy/delete: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "name": req.Name})
}
