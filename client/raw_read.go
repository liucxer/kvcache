package client

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
)

// 裸 TCP 数据面客户端：读大 value 走实例的 sendfile 端口（注册表 raw_addr，
// 缺省回退 gRPC 端口 +2）。相比 gRPC Get：服务端零用户态拷贝（sendfile），
// 客户端直接读进调用方提供的 buffer——消除每次 4MB 分配+清零（pprof 实测
// 占读路径 SDK 侧 CPU 的 15%）和 proto/帧拼装拷贝。

type rawConn struct {
	conn net.Conn
	mu   sync.Mutex // 每连接同时只允许一个请求（协议无请求 ID）
}

// GetInto 读取 key 的 value 写入调用方提供的 buf，返回实际长度。
// 路由逻辑与 Get 一致；buf 容量不足时返回错误（连接已排空，可继续使用）。
// 直连模式下跳过 cache/index，直接访问 DirectAddr 实例。
func (c *Client) GetInto(ctx context.Context, key string, buf []byte) (int, error) {
	if c.directMode {
		return c.getIntoFromDirect(key, buf)
	}

	if instName, ok := c.cache.Get(key); ok {
		if n, err := c.getIntoFromInstance(instName, key, buf); err == nil {
			return n, nil
		} else if err == ErrKeyNotFound || isInstanceOffline(err) {
			c.cache.Delete(key)
		} else {
			return 0, err
		}
	}

	indexData, err := c.index.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	if indexData == nil {
		return 0, ErrKeyNotFound
	}

	c.cache.Put(key, indexData.Instance)

	n, err := c.getIntoFromInstance(indexData.Instance, key, buf)
	if err != nil {
		if err == ErrKeyNotFound || isInstanceOffline(err) {
			c.cache.Delete(key)
		}
		return 0, err
	}
	return n, nil
}

// getIntoFromDirect 直连模式裸 TCP 读：跳过 registry/cache/index，
// 直接用 directInst 的 raw 数据面地址
func (c *Client) getIntoFromDirect(key string, buf []byte) (int, error) {
	rawAddr, err := resolveRawAddr(c.directInst)
	if err != nil {
		return 0, err
	}
	rc, err := c.getRawConn(rawAddr)
	if err != nil {
		return 0, err
	}

	rc.mu.Lock()
	defer rc.mu.Unlock()

	// 请求: [4B keyLen][key]
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(key)))
	if _, err := rc.conn.Write(hdr[:]); err != nil {
		c.resetRawConn(rawAddr, rc)
		return 0, err
	}
	if _, err := rc.conn.Write([]byte(key)); err != nil {
		c.resetRawConn(rawAddr, rc)
		return 0, err
	}

	// 响应头: [1B status][8B bodyLen]
	var respHdr [9]byte
	if _, err := io.ReadFull(rc.conn, respHdr[:]); err != nil {
		c.resetRawConn(rawAddr, rc)
		return 0, err
	}
	status := respHdr[0]
	bodyLen := int64(binary.BigEndian.Uint64(respHdr[1:]))

	switch status {
	case 1: // not found
		return 0, ErrKeyNotFound
	case 2: // error
		msg := make([]byte, bodyLen)
		if _, err := io.ReadFull(rc.conn, msg); err != nil {
			c.resetRawConn(rawAddr, rc)
			return 0, err
		}
		return 0, fmt.Errorf("raw read error from direct: %s", string(msg))
	case 0:
		// ok
	default:
		c.resetRawConn(rawAddr, rc)
		return 0, fmt.Errorf("raw read: bad status %d from direct", status)
	}

	if int64(len(buf)) < bodyLen {
		io.CopyN(io.Discard, rc.conn, bodyLen)
		return 0, fmt.Errorf("raw read: buffer too small (%d < %d)", len(buf), bodyLen)
	}
	if _, err := io.ReadFull(rc.conn, buf[:bodyLen]); err != nil {
		c.resetRawConn(rawAddr, rc)
		return 0, err
	}
	return int(bodyLen), nil
}

func (c *Client) getIntoFromInstance(instName, key string, buf []byte) (int, error) {
	insts := c.registry.GetActiveInstances()
	inst, ok := insts[instName]
	if !ok {
		return 0, fmt.Errorf("instance %s is offline", instName)
	}

	rawAddr, err := resolveRawAddr(inst)
	if err != nil {
		return 0, err
	}
	rc, err := c.getRawConn(rawAddr)
	if err != nil {
		return 0, err
	}

	rc.mu.Lock()
	defer rc.mu.Unlock()

	// 请求: [4B keyLen][key]
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(key)))
	if _, err := rc.conn.Write(hdr[:]); err != nil {
		c.resetRawConn(rawAddr, rc)
		return 0, err
	}
	if _, err := rc.conn.Write([]byte(key)); err != nil {
		c.resetRawConn(rawAddr, rc)
		return 0, err
	}

	// 响应头: [1B status][8B bodyLen]
	var respHdr [9]byte
	if _, err := io.ReadFull(rc.conn, respHdr[:]); err != nil {
		c.resetRawConn(rawAddr, rc)
		return 0, err
	}
	status := respHdr[0]
	bodyLen := int64(binary.BigEndian.Uint64(respHdr[1:]))

	switch status {
	case 1: // not found
		return 0, ErrKeyNotFound
	case 2: // error
		msg := make([]byte, bodyLen)
		if _, err := io.ReadFull(rc.conn, msg); err != nil {
			c.resetRawConn(rawAddr, rc)
			return 0, err
		}
		return 0, fmt.Errorf("raw read error from %s: %s", instName, string(msg))
	case 0:
		// ok，继续
	default:
		c.resetRawConn(rawAddr, rc)
		return 0, fmt.Errorf("raw read: bad status %d from %s", status, instName)
	}

	if int64(len(buf)) < bodyLen {
		// 排空连接保持可用，再报错
		io.CopyN(io.Discard, rc.conn, bodyLen)
		return 0, fmt.Errorf("raw read: buffer too small (%d < %d)", len(buf), bodyLen)
	}
	if _, err := io.ReadFull(rc.conn, buf[:bodyLen]); err != nil {
		c.resetRawConn(rawAddr, rc)
		return 0, err
	}
	return int(bodyLen), nil
}

// resolveRawAddr 解析实例裸数据面地址：注册表 raw_addr 优先，
// 旧实例（无 raw_addr）回退为 gRPC 端口 +2；host 为空视为本机。
func resolveRawAddr(inst *InstanceInfo) (string, error) {
	addr := inst.RawAddr
	if addr == "" {
		host, portStr, err := net.SplitHostPort(inst.Addr)
		if err != nil {
			return "", fmt.Errorf("bad instance addr %q: %v", inst.Addr, err)
		}
		port, err := strconv.Atoi(strings.TrimSpace(portStr))
		if err != nil {
			return "", fmt.Errorf("bad instance addr %q: %v", inst.Addr, err)
		}
		addr = net.JoinHostPort(host, strconv.Itoa(port+2))
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("bad raw addr %q: %v", addr, err)
	}
	if host == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port), nil
}

func (c *Client) getRawConn(rawAddr string) (*rawConn, error) {
	c.connMu.RLock()
	rc, ok := c.rawConns[rawAddr]
	c.connMu.RUnlock()
	if ok {
		return rc, nil
	}

	c.connMu.Lock()
	defer c.connMu.Unlock()
	if rc, ok := c.rawConns[rawAddr]; ok {
		return rc, nil
	}

	conn, err := net.Dial("tcp", rawAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial raw data %s: %v", rawAddr, err)
	}
	if tc, ok := conn.(*net.TCPConn); ok {
		tc.SetNoDelay(true)
	}

	rc = &rawConn{conn: conn}
	c.rawConns[rawAddr] = rc
	return rc, nil
}

func (c *Client) resetRawConn(rawAddr string, rc *rawConn) {
	rc.conn.Close()
	c.connMu.Lock()
	if c.rawConns[rawAddr] == rc {
		delete(c.rawConns, rawAddr)
	}
	c.connMu.Unlock()
}
