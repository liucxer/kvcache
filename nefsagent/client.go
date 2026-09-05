package nefsagent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Client nefsagent 客户端，维持长连接池
type Client struct {
	addr   string
	token  string
	pool   chan *conn
	closed int32
}

type conn struct {
	c       net.Conn
	pending map[uint64]chan *Frame
	mu      sync.Mutex
	writeMu sync.Mutex
	nextID  uint64
	closed  int32
}

// ClientOption 配置 Client
type ClientOption func(*Client)

// WithPoolSize 设置长连接池大小（默认 4）
func WithPoolSize(n int) ClientOption {
	return func(c *Client) {
		if n < 1 {
			n = 1
		}
		c.pool = make(chan *conn, n)
	}
}

// NewClient 创建客户端并建立连接池
func NewClient(addr, token string, opts ...ClientOption) (*Client, error) {
	c := &Client{
		addr:   addr,
		token:  token,
		pool:   make(chan *conn, 4),
	}
	for _, o := range opts {
		o(c)
	}
	// 预建连接
	size := cap(c.pool)
	for i := 0; i < size; i++ {
		conn, err := c.dial()
		if err != nil {
			// 失败时退化为懒连接
			break
		}
		c.pool <- conn
	}
	return c, nil
}

func (c *Client) dial() (*conn, error) {
	nc, err := net.DialTimeout("tcp", c.addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	conn := &conn{
		c:       nc,
		pending: make(map[uint64]chan *Frame),
	}
	// 必须先启动 readLoop，否则 auth 的 roundTrip 发帧后没人读响应会超时
	go conn.readLoop()
	// 鉴权
	if err := conn.auth(c.token); err != nil {
		conn.markClosed()
		return nil, err
	}
	return conn, nil
}

func (conn *conn) auth(token string) error {
	req := &AuthReq{Token: token}
	payload, _ := json.Marshal(req)
	resp, err := conn.roundTrip(MsgAuthReq, payload, 3*time.Second)
	if err != nil {
		return err
	}
	var ar AuthResp
	if err := json.Unmarshal(resp.Payload, &ar); err != nil {
		return err
	}
	if !ar.OK {
		return fmt.Errorf("nefsagent: auth failed: %s", ar.Error)
	}
	return nil
}

// readLoop 持续读帧并按 reqID 分发
func (conn *conn) readLoop() {
	for {
		frame, err := DecodeFrame(conn.c)
		if err != nil {
			conn.markClosed()
			return
		}
		conn.mu.Lock()
		ch, ok := conn.pending[frame.ReqID]
		if ok {
			delete(conn.pending, frame.ReqID)
		}
		conn.mu.Unlock()
		if ok {
			ch <- frame
		}
	}
}

func (conn *conn) markClosed() {
	atomic.StoreInt32(&conn.closed, 1)
	conn.c.Close()
	conn.mu.Lock()
	for id, ch := range conn.pending {
		delete(conn.pending, id)
		close(ch)
	}
	conn.mu.Unlock()
}

// roundTrip 发送请求并等待响应
func (conn *conn) roundTrip(msgType byte, payload []byte, timeout time.Duration) (*Frame, error) {
	if atomic.LoadInt32(&conn.closed) == 1 {
		return nil, fmt.Errorf("nefsagent: conn closed")
	}
	reqID := atomic.AddUint64(&conn.nextID, 1)
	ch := make(chan *Frame, 1)

	conn.mu.Lock()
	conn.pending[reqID] = ch
	conn.mu.Unlock()

	conn.writeMu.Lock()
	_, err := conn.c.Write((&Frame{ReqID: reqID, MsgType: msgType, Payload: payload}).Encode())
	conn.writeMu.Unlock()
	if err != nil {
		conn.markClosed()
		return nil, err
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("nefsagent: conn closed waiting for resp")
		}
		return resp, nil
	case <-timer.C:
		conn.mu.Lock()
		delete(conn.pending, reqID)
		conn.mu.Unlock()
		return nil, fmt.Errorf("nefsagent: request timeout")
	}
}

// getConn 从连接池取一个连接，失败则重建
func (c *Client) getConn(ctx context.Context) (*conn, error) {
	select {
	case conn := <-c.pool:
		if atomic.LoadInt32(&conn.closed) == 1 {
			// 重建
			newConn, err := c.dial()
			if err != nil {
				c.pool <- nil // 占位避免空读
				return nil, err
			}
			return newConn, nil
		}
		return conn, nil
	default:
		// 池空了，新建（但不归还池，用完就扔）
		return c.dial()
	}
}

// putConn 归还连接
func (c *Client) putConn(conn *conn) {
	if conn == nil || atomic.LoadInt32(&conn.closed) == 1 {
		return
	}
	select {
	case c.pool <- conn:
	default:
		// 池满，丢弃
		conn.c.Close()
	}
}

// Exec 执行命令
func (c *Client) Exec(ctx context.Context, cmd string, timeout time.Duration, cwd string) (*ExecResp, error) {
	req := &ExecReq{Cmd: cmd}
	if timeout > 0 {
		req.Timeout = timeout.Seconds()
	}
	req.Cwd = cwd
	payload, _ := json.Marshal(req)

	conn, err := c.getConn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.putConn(conn)

	// roundTrip 超时 = 命令超时 + 缓冲；无显式超时给 2h 兜底（服务端默认 1h）
	rt := 2 * time.Hour
	if timeout > 0 {
		rt = timeout + 30*time.Second
	}
	resp, err := conn.roundTrip(MsgExecReq, payload, rt)
	if err != nil {
		return nil, err
	}
	if resp.MsgType == MsgError {
		var er ErrorResp
		json.Unmarshal(resp.Payload, &er)
		return nil, fmt.Errorf("nefsagent: exec error: %s", er.Error)
	}
	var er ExecResp
	if err := json.Unmarshal(resp.Payload, &er); err != nil {
		return nil, err
	}
	return &er, nil
}

// ExecOut 便捷封装：返回 stdout
func (c *Client) ExecOut(ctx context.Context, cmd string) (string, error) {
	resp, err := c.Exec(ctx, cmd, 0, "")
	if err != nil {
		return "", err
	}
	if resp.TimedOut {
		return "", fmt.Errorf("nefsagent: exec timed out: %s", cmd)
	}
	if resp.ExitCode != 0 {
		return resp.Stdout, fmt.Errorf("nefsagent: exec exit=%d stderr=%s", resp.ExitCode, resp.Stderr)
	}
	return resp.Stdout, nil
}

// Health 健康检查
func (c *Client) Health(ctx context.Context) error {
	conn, err := c.getConn(ctx)
	if err != nil {
		return err
	}
	defer c.putConn(conn)

	resp, err := conn.roundTrip(MsgHealthReq, nil, 5*time.Second)
	if err != nil {
		return err
	}
	var hr HealthResp
	if err := json.Unmarshal(resp.Payload, &hr); err != nil {
		return err
	}
	if !hr.OK {
		return fmt.Errorf("nefsagent: health: %s", hr.Error)
	}
	return nil
}

// UploadFile 上传本地文件
func (c *Client) UploadFile(ctx context.Context, localPath, remotePath string) (int64, error) {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return 0, err
	}
	return c.Upload(ctx, remotePath, data)
}

// Upload 上传字节
func (c *Client) Upload(ctx context.Context, remotePath string, data []byte) (int64, error) {
	req := &UploadReq{Path: remotePath, Size: int64(len(data))}
	header, _ := json.Marshal(req)
	payload := append(header, data...)

	conn, err := c.getConn(ctx)
	if err != nil {
		return 0, err
	}
	defer c.putConn(conn)

	resp, err := conn.roundTrip(MsgUploadReq, payload, 60*time.Second)
	if err != nil {
		return 0, err
	}
	if resp.MsgType == MsgError {
		var er ErrorResp
		json.Unmarshal(resp.Payload, &er)
		return 0, fmt.Errorf("nefsagent: upload error: %s", er.Error)
	}
	var ur UploadResp
	if err := json.Unmarshal(resp.Payload, &ur); err != nil {
		return 0, err
	}
	if ur.Code != 0 {
		return ur.Size, fmt.Errorf("nefsagent: upload failed: %s", ur.Error)
	}
	return ur.Size, nil
}

// DownloadFile 下载到本地
func (c *Client) DownloadFile(ctx context.Context, remotePath, localPath string) error {
	data, err := c.Download(ctx, remotePath)
	if err != nil {
		return err
	}
	if dir := dir(localPath); dir != "" && dir != "." {
		os.MkdirAll(dir, 0755)
	}
	return os.WriteFile(localPath, data, 0644)
}

// Download 下载字节
func (c *Client) Download(ctx context.Context, remotePath string) ([]byte, error) {
	req := &DownloadReq{Path: remotePath}
	payload, _ := json.Marshal(req)

	conn, err := c.getConn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.putConn(conn)

	resp, err := conn.roundTrip(MsgDownloadReq, payload, 60*time.Second)
	if err != nil {
		return nil, err
	}
	if resp.MsgType == MsgError {
		var er ErrorResp
		json.Unmarshal(resp.Payload, &er)
		return nil, fmt.Errorf("nefsagent: download error: %s", er.Error)
	}
	// 响应 payload = JSON(DownloadResp) + raw bytes
	end := -1
	for i := 0; i < len(resp.Payload); i++ {
		if resp.Payload[i] == '}' {
			end = i + 1
			break
		}
	}
	if end < 0 {
		return nil, fmt.Errorf("nefsagent: download bad resp")
	}
	var dr DownloadResp
	if err := json.Unmarshal(resp.Payload[:end], &dr); err != nil {
		return nil, err
	}
	if dr.Code != 0 {
		return nil, fmt.Errorf("nefsagent: download failed: %s", dr.Error)
	}
	return resp.Payload[end:], nil
}

// Close 关闭所有连接
func (c *Client) Close() error {
	atomic.StoreInt32(&c.closed, 1)
	for {
		select {
		case conn := <-c.pool:
			if conn != nil {
				conn.c.Close()
			}
		default:
			return nil
		}
	}
}

func dir(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[:i]
		}
	}
	return ""
}

var _ = io.EOF
