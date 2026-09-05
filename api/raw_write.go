package api

import (
	"context"
	"encoding/binary"
	"io"
	"log"
	"net"
	"time"

	"kvcache/service"
)

// 裸 TCP 写数据面：大 value 收包后直接写 RocksDB，不做 gRPC/HTTP2 分帧，
// 配合客户端 sendmsg/sendmmsg 批量投递，省掉逐包系统调用与帧解析拷贝。
// 与读数据面（raw_data.go）对称，独立端口（--raw-write-addr）。
//
// 协议（大端，连接持久，支持流水线——客户端可连发多帧不等 ACK）：
//
//	请求:  [4B keyLen][key][8B valueLen][value]
//	响应:  [1B status][8B bodyLen][body（仅 status=2 错误时有）]
//	status: 0=ok，2=error（body 为错误信息）
type RawWriteServer struct {
	svc  *service.KVService
	ln   net.Listener
	done chan struct{}
}

const (
	rawWriteMaxKeyLen   = 64 * 1024
	rawWriteMaxValueLen = 64 * 1024 * 1024
)

// StartRawWriteServer 在 addr 上启动裸 TCP 写数据面服务
func StartRawWriteServer(addr string, svc *service.KVService) (*RawWriteServer, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := &RawWriteServer{svc: svc, ln: ln, done: make(chan struct{})}
	go s.acceptLoop()
	log.Printf("Raw write server started on %s", addr)
	return s, nil
}

func (s *RawWriteServer) Close() {
	close(s.done)
	s.ln.Close()
}

// Addr 返回实际监听地址
func (s *RawWriteServer) Addr() string {
	return s.ln.Addr().String()
}

func (s *RawWriteServer) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			select {
			case <-s.done:
				return
			default:
				log.Printf("raw write accept error: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		if tc, ok := conn.(*net.TCPConn); ok {
			tc.SetNoDelay(true)
		}
		go s.serveConn(conn)
	}
}

// serveConn 循环处理一个持久连接上的写帧；单连接内按序处理以保证 ACK 顺序。
func (s *RawWriteServer) serveConn(conn net.Conn) {
	defer conn.Close()

	hdr := make([]byte, 12)
	var key []byte
	var value []byte
	for {
		if _, err := io.ReadFull(conn, hdr); err != nil {
			return // 对端关闭或读失败，直接断开
		}
		keyLen := binary.BigEndian.Uint32(hdr[0:4])
		valueLen := binary.BigEndian.Uint64(hdr[4:12])
		if keyLen == 0 || keyLen > rawWriteMaxKeyLen ||
			valueLen == 0 || valueLen > rawWriteMaxValueLen {
			return // 非法帧头，断开
		}

		if cap(key) < int(keyLen) {
			key = make([]byte, keyLen)
		}
		key = key[:keyLen]
		if _, err := io.ReadFull(conn, key); err != nil {
			return
		}
		if cap(value) < int(valueLen) {
			value = make([]byte, valueLen)
		}
		value = value[:valueLen]
		if _, err := io.ReadFull(conn, value); err != nil {
			return
		}

		ack := make([]byte, 9)
		if err := s.svc.Set(context.Background(), string(key), value, 0); err != nil {
			msg := []byte(err.Error())
			ack[0] = rawStatusError
			binary.BigEndian.PutUint64(ack[1:], uint64(len(msg)))
			if _, werr := conn.Write(ack); werr != nil {
				return
			}
			if _, werr := conn.Write(msg); werr != nil {
				return
			}
			continue
		}

		ack[0] = rawStatusOK
		binary.BigEndian.PutUint64(ack[1:], 0)
		if _, werr := conn.Write(ack); werr != nil {
			return
		}
	}
}
