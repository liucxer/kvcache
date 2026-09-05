package nefsagent

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// ProxyRule 一条动态 TCP 端口转发规则（与 Python proxy.py 的 /proxy 接口字段一致）
type ProxyRule struct {
	Name       string `json:"name"`
	ListenIP   string `json:"listen_ip"`
	ListenPort int    `json:"listen_port"`
	TargetIP   string `json:"target_ip"`
	TargetPort int    `json:"target_port"`
	Backlog    int    `json:"backlog,omitempty"`
	Alive      bool   `json:"alive"`

	ln net.Listener `json:"-"`
}

// ProxyManager 管理节点上的动态端口转发规则
type ProxyManager struct {
	mu    sync.RWMutex
	rules map[string]*ProxyRule
}

// NewProxyManager 创建规则管理器
func NewProxyManager() *ProxyManager {
	return &ProxyManager{rules: make(map[string]*ProxyRule)}
}

// Add 新增转发规则；name 为空时自动生成。成功后立即开始监听并转发。
func (m *ProxyManager) Add(rule *ProxyRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rule.Name == "" {
		rule.Name = fmt.Sprintf("rule-%d", time.Now().UnixNano())
	}
	if _, exists := m.rules[rule.Name]; exists {
		return fmt.Errorf("proxy: rule %s already exists", rule.Name)
	}
	if rule.ListenPort <= 0 {
		return fmt.Errorf("proxy: invalid listen port %d", rule.ListenPort)
	}
	if rule.TargetIP == "" || rule.TargetPort <= 0 {
		return fmt.Errorf("proxy: invalid target %s:%d", rule.TargetIP, rule.TargetPort)
	}
	if rule.ListenIP == "" {
		rule.ListenIP = "0.0.0.0"
	}
	if rule.Backlog <= 0 {
		rule.Backlog = 128
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", rule.ListenIP, rule.ListenPort))
	if err != nil {
		return fmt.Errorf("proxy: listen %s:%d: %w", rule.ListenIP, rule.ListenPort, err)
	}
	rule.ln = ln
	rule.Alive = true
	m.rules[rule.Name] = rule
	go m.serve(rule)
	return nil
}

// serve 接受连接并转发到目标
func (m *ProxyManager) serve(rule *ProxyRule) {
	for {
		conn, err := rule.ln.Accept()
		if err != nil {
			return // 监听被关闭（规则删除）
		}
		go func(c net.Conn) {
			defer c.Close()
			target, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", rule.TargetIP, rule.TargetPort), 10*time.Second)
			if err != nil {
				return
			}
			defer target.Close()
			proxyPipe(c, target)
		}(conn)
	}
}

// Remove 删除规则并关闭监听
func (m *ProxyManager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rule, ok := m.rules[name]
	if !ok {
		return fmt.Errorf("proxy: rule %s not found", name)
	}
	delete(m.rules, name)
	rule.Alive = false
	if rule.ln != nil {
		return rule.ln.Close()
	}
	return nil
}

// List 返回所有规则副本
func (m *ProxyManager) List() []ProxyRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ProxyRule, 0, len(m.rules))
	for _, r := range m.rules {
		out = append(out, *r)
	}
	return out
}

// proxyPipe 双向转发两条连接，任一方向结束即关闭双方
func proxyPipe(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(a, b)
		closeWrite(a)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(b, a)
		closeWrite(b)
		done <- struct{}{}
	}()
	<-done
	a.Close()
	b.Close()
	<-done
}

func closeWrite(c net.Conn) {
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.CloseWrite()
	} else {
		_ = c.Close()
	}
}
