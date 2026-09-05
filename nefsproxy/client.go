// Package nefsproxy 是 nefs-proxy (proxy.py) HTTP 服务的 Go 客户端。
//
// proxy.py 运行在目标机上（默认 :9527，token 95279527），提供：
//   - /exec         远程命令执行
//   - /upload /download /ls /mkdir /delete   文件传输
//   - /proxy*       动态 TCP 端口转发规则管理
//   - /ping         健康检查
//
// 本客户端纯标准库实现，CGO_ENABLED=0 即可编译，可被 bench/test 工具
// 直接 import 取代 paramiko/sshpass。
package nefsproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultTimeout = 3600 * time.Second
)

// APIError 表示 proxy 服务端返回的业务错误（HTTP 200 但 code != 0）
// 或 HTTP 非 2xx 响应。
type APIError struct {
	HTTPStatus int    `json:"-"`
	Code       int    `json:"code"`
	Message    string `json:"error,omitempty"`
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("nefsproxy: api error (http=%d code=%d): %s", e.HTTPStatus, e.Code, e.Message)
	}
	return fmt.Sprintf("nefsproxy: api error (http=%d code=%d)", e.HTTPStatus, e.Code)
}

// ExecResult /exec 响应
type ExecResult struct {
	Code       int    `json:"code"`
	Cmd        string `json:"cmd"`
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	TimedOut   bool   `json:"timed_out"`
	ElapsedMs  int    `json:"elapsed_ms"`
}

// Entry /ls 单个目录项
type Entry struct {
	Name  string `json:"name"`
	Type  string `json:"type"` // "dir" | "file"
	Size  int64  `json:"size"`
	Mtime int64  `json:"mtime"`
}

// ProxyRule 端口转发规则
type ProxyRule struct {
	Name       string `json:"name"`
	ListenIP   string `json:"listen_ip"`
	ListenPort int    `json:"listen_port"`
	TargetIP   string `json:"target_ip"`
	TargetPort int    `json:"target_port"`
	Alive      bool   `json:"alive,omitempty"`
}

// Client 是 nefs-proxy 服务的客户端。
type Client struct {
	baseURL    string // 形如 http://host:9527
	token      string
	httpClient *http.Client
}

// Option 配置 Client
type Option func(*Client)

// WithHTTPClient 注入自定义 *http.Client（用于 TLS、超时、代理等）
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// WithToken 覆盖默认 token
func WithToken(token string) Option {
	return func(c *Client) { c.token = token }
}

// WithTimeout 设置默认请求超时
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = d }
}

// NewClient 创建客户端。baseURL 形如 "http://100.71.128.13:9527"。
// token 为空时使用 proxy.py 默认 token "95279527"。
func NewClient(baseURL, token string, opts ...Option) *Client {
	if token == "" {
		token = "95279527"
	}
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// ---- 底层 HTTP ----

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Token", c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// doJSON 发送请求并把 JSON 响应解码到 out（out 可为 nil）。
// HTTP 非 2xx 或 code != 0 时返回 *APIError。
func (c *Client) doJSON(ctx context.Context, method, path string, body io.Reader, out any) error {
	req, err := c.newRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("nefsproxy: http do %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("nefsproxy: read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var ae APIError
		_ = json.Unmarshal(raw, &ae)
		ae.HTTPStatus = resp.StatusCode
		if ae.Message == "" {
			ae.Message = string(raw)
		}
		return &ae
	}

	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("nefsproxy: decode json: %w (body=%s)", err, truncate(raw, 200))
		}
		// 业务 code 检查
		type codeCarrier struct {
			Code int `json:"code"`
		}
		var cc codeCarrier
		_ = json.Unmarshal(raw, &cc)
		if cc.Code != 0 {
			var ae APIError
			_ = json.Unmarshal(raw, &ae)
			ae.HTTPStatus = resp.StatusCode
			if ae.Message == "" {
				ae.Message = string(raw)
			}
			return &ae
		}
	}
	return nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}

// ---- 命令执行 ----

// Exec 在远程执行 shell 命令，返回完整结果。
// timeout 为 0 时使用 proxy 服务端默认值（3600s）。
func (c *Client) Exec(ctx context.Context, cmd string, timeout time.Duration, cwd string) (*ExecResult, error) {
	if cmd == "" {
		return nil, fmt.Errorf("nefsproxy: Exec cmd is empty")
	}
	body := map[string]any{"cmd": cmd}
	if timeout > 0 {
		body["timeout"] = timeout.Seconds()
	}
	if cwd != "" {
		body["cwd"] = cwd
	}
	raw, _ := json.Marshal(body)

	var res ExecResult
	if err := c.doJSON(ctx, http.MethodPost, "/exec", bytes.NewReader(raw), &res); err != nil {
		return nil, err
	}
	if res.Code != 0 {
		// 超时等业务失败也带回来，不报错
		return &res, nil
	}
	return &res, nil
}

// ExecOut 是 Exec 的便捷封装：返回 stdout 字符串。
// 命令非 0 退出时返回 stderr 作为 error。
func (c *Client) ExecOut(ctx context.Context, cmd string) (string, error) {
	res, err := c.Exec(ctx, cmd, 0, "")
	if err != nil {
		return "", err
	}
	if res.TimedOut {
		return "", fmt.Errorf("nefsproxy: exec timed out: %s", cmd)
	}
	if res.ExitCode != 0 {
		return res.Stdout, fmt.Errorf("nefsproxy: exec exit=%d stderr=%s", res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	return res.Stdout, nil
}

// ExecBatchResult 批量执行结果
type ExecBatchResult struct {
	// 每个命令的 stdout（按输入顺序）
	Stdouts []string
	// 每个命令的 exit code（按输入顺序）
	ExitCodes []int
	// 聚合的整体结果（来自服务端）
	Raw *ExecResult
}

// ExecBatch 把多个命令合并成一次 HTTP /exec 请求发送，摊薄 TCP 握手开销。
//
// 工作原理：用唯一分隔符把命令串成一个 shell 脚本，每个命令后打印分隔符
// 及其 exit code，再解析拆分。适用于 proxy.py 不支持 keep-alive 时的高频调用。
//
// 注意：这是 best-effort 解析——若命令 stdout 恰好包含分隔符文本，可能
// 拆分错误；生产场景建议配合 Exec 单调用或改服务端支持 keep-alive。
func (c *Client) ExecBatch(ctx context.Context, cmds []string, timeout time.Duration, cwd string) (*ExecBatchResult, error) {
	if len(cmds) == 0 {
		return &ExecBatchResult{}, nil
	}
	if len(cmds) == 1 {
		res, err := c.Exec(ctx, cmds[0], timeout, cwd)
		if err != nil {
			return nil, err
		}
		return &ExecBatchResult{
			Stdouts:   []string{res.Stdout},
			ExitCodes: []int{res.ExitCode},
			Raw:       res,
		}, nil
	}

	// 唯一分隔符：足够随机避免冲突
	const delim = "__NEFS_BATCH_DELIM_a7f3b2c1__"
	parts := make([]string, 0, len(cmds)*2+1)
	parts = append(parts, "set +e") // 忽略单条命令失败，继续执行后续
	for _, cmd := range cmds {
		parts = append(parts, cmd)
		// 打印分隔符 + exit code，便于解析
		parts = append(parts, fmt.Sprintf(`echo "%s:$?"`, delim))
	}
	combined := strings.Join(parts, "; ")

	res, err := c.Exec(ctx, combined, timeout, cwd)
	if err != nil {
		return nil, err
	}

	// 解析：分隔符行形如 __NEFS_BATCH_DELIM_a7f3b2c1__:0
	lines := strings.Split(res.Stdout, "\n")
	var stdouts []string
	var codes []int
	var cur strings.Builder
	for _, line := range lines {
		idx := strings.Index(line, delim)
		if idx >= 0 {
			// 找到分隔符行，提取 exit code
			rest := line[idx+len(delim):]
			rest = strings.TrimPrefix(rest, ":")
			var code int
			fmt.Sscanf(rest, "%d", &code)
			stdouts = append(stdouts, cur.String())
			codes = append(codes, code)
			cur.Reset()
		} else {
			if cur.Len() > 0 {
				cur.WriteByte('\n')
			}
			cur.WriteString(line)
		}
	}
	// 若最后一条命令后没有分隔符（理论上不会），补一条
	if cur.Len() > 0 || len(stdouts) < len(cmds) {
		stdouts = append(stdouts, cur.String())
		codes = append(codes, res.ExitCode)
	}

	return &ExecBatchResult{
		Stdouts:   stdouts,
		ExitCodes: codes,
		Raw:       res,
	}, nil
}

// ---- 文件传输 ----

// Upload 把 src 的内容上传到远程路径 path。
// path 为绝对路径；相对路径会拼到 proxy 服务端 --root。
func (c *Client) Upload(ctx context.Context, path string, src io.Reader) (size int64, err error) {
	if path == "" {
		return 0, fmt.Errorf("nefsproxy: Upload path is empty")
	}
	u := c.baseURL + "/upload?path=" + url.QueryEscape(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, src)
	if err != nil {
		return 0, err
	}
	req.Header.Set("X-Token", c.token)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("nefsproxy: upload: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("nefsproxy: upload read: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, &APIError{HTTPStatus: resp.StatusCode, Message: string(raw)}
	}
	var out struct {
		Code int    `json:"code"`
		Path string `json:"path"`
		Size int64  `json:"size"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return 0, fmt.Errorf("nefsproxy: upload decode: %w", err)
	}
	if out.Code != 0 {
		return out.Size, &APIError{Code: out.Code, Message: string(raw)}
	}
	return out.Size, nil
}

// UploadFile 上传本地文件到远程路径。
func (c *Client) UploadFile(ctx context.Context, localPath, remotePath string) (int64, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return c.Upload(ctx, remotePath, f)
}

// Download 下载远程文件，返回 io.ReadCloser（调用方负责 Close）。
func (c *Client) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	if path == "" {
		return nil, fmt.Errorf("nefsproxy: Download path is empty")
	}
	u := c.baseURL + "/download?path=" + url.QueryEscape(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Token", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nefsproxy: download: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, &APIError{HTTPStatus: resp.StatusCode, Message: string(raw)}
	}
	return resp.Body, nil
}

// DownloadFile 下载远程文件保存到本地。
func (c *Client) DownloadFile(ctx context.Context, remotePath, localPath string) error {
	rc, err := c.Download(ctx, remotePath)
	if err != nil {
		return err
	}
	defer rc.Close()

	if dir := filepath.Dir(localPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	f, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, rc)
	return err
}

// Ls 列目录
func (c *Client) Ls(ctx context.Context, path string) ([]Entry, error) {
	u := "/ls?path=" + url.QueryEscape(path)
	var out struct {
		Code    int     `json:"code"`
		Path    string  `json:"path"`
		Entries []Entry `json:"entries"`
	}
	if err := c.doJSON(ctx, http.MethodGet, u, nil, &out); err != nil {
		return nil, err
	}
	return out.Entries, nil
}

// Mkdir 远程创建目录（递归）
func (c *Client) Mkdir(ctx context.Context, path string) error {
	return c.doJSON(ctx, http.MethodPost, "/mkdir?path="+url.QueryEscape(path), nil, nil)
}

// Delete 远程删除文件或目录
func (c *Client) Delete(ctx context.Context, path string) error {
	return c.doJSON(ctx, http.MethodPost, "/delete?path="+url.QueryEscape(path), nil, nil)
}

// ---- 健康检查 ----

// Ping 健康检查（proxy.py 不要求 token）
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/ping", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{HTTPStatus: resp.StatusCode, Message: "ping failed"}
	}
	return nil
}

// ---- 端口转发规则管理 ----

// ProxyList 列出当前所有转发规则
func (c *Client) ProxyList(ctx context.Context) ([]ProxyRule, error) {
	var out struct {
		Code  int         `json:"code"`
		Rules []ProxyRule `json:"rules"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/proxy", nil, &out); err != nil {
		return nil, err
	}
	return out.Rules, nil
}

// ProxyAdd 新增转发规则。name 为空时由服务端自动生成。
func (c *Client) ProxyAdd(ctx context.Context, rule ProxyRule) error {
	body, _ := json.Marshal(rule)
	return c.doJSON(ctx, http.MethodPost, "/proxy/add", bytes.NewReader(body), nil)
}

// ProxyDelete 删除转发规则
func (c *Client) ProxyDelete(ctx context.Context, name string) error {
	body, _ := json.Marshal(map[string]string{"name": name})
	return c.doJSON(ctx, http.MethodPost, "/proxy/delete", bytes.NewReader(body), nil)
}

// DialTunnel 通过 proxy 转发规则建立到目标的 TCP 连接。
//
// 流程：先 /proxy/add 创建规则（listen_port 由 proxy 服务端所在机监听，
// 转发到 targetIP:targetPort），然后本机 dial 到 proxyHost:listenPort。
// cleanup 函数调用时删除该规则（建议 defer）。
//
// proxyHost 是 proxy.py 所在机的可达地址（不含端口），如 "100.71.128.13"。
// listenPort 为 0 表示由客户端随机选一个端口；若想固定可显式指定。
func (c *Client) DialTunnel(ctx context.Context, proxyHost string, listenPort int, targetIP string, targetPort int) (conn net.Conn, cleanup func(), err error) {
	if targetIP == "" || targetPort <= 0 {
		return nil, nil, fmt.Errorf("nefsproxy: tunnel target invalid: %s:%d", targetIP, targetPort)
	}
	if proxyHost == "" {
		return nil, nil, fmt.Errorf("nefsproxy: proxyHost is empty")
	}

	// 随机端口
	if listenPort <= 0 {
		ln, lerr := net.Listen("tcp", "0.0.0.0:0")
		if lerr != nil {
			return nil, nil, fmt.Errorf("nefsproxy: pick port: %w", lerr)
		}
		listenPort = ln.Addr().(*net.TCPAddr).Port
		ln.Close()
	}

	rule := ProxyRule{
		Name:       fmt.Sprintf("go-tunnel-%d", time.Now().UnixNano()),
		ListenIP:   "0.0.0.0",
		ListenPort: listenPort,
		TargetIP:   targetIP,
		TargetPort: targetPort,
	}
	if err := c.ProxyAdd(ctx, rule); err != nil {
		return nil, nil, fmt.Errorf("nefsproxy: tunnel add rule: %w", err)
	}

	cleanup = func() {
		// 用独立 context，调用方 ctx 可能已取消
		delCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.ProxyDelete(delCtx, rule.Name)
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err = dialer.DialContext(ctx, "tcp", net.JoinHostPort(proxyHost, fmt.Sprintf("%d", listenPort)))
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("nefsproxy: tunnel dial %s:%d: %w", proxyHost, listenPort, err)
	}
	return conn, cleanup, nil
}
