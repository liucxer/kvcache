package client

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"kvcache/proto"
)

// Client KVCache客户端

type Client struct {
	clients []proto.KeyValueServiceClient
	mutex   sync.Mutex
	index   int
}

// NewClient 创建一个新的客户端
func NewClient(addrs []string) (*Client, error) {
	clients := make([]proto.KeyValueServiceClient, 0, len(addrs))

	for _, addr := range addrs {
		// 创建gRPC连接
		conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Failed to connect to %s: %v", addr, err)
			continue
		}

		// 创建客户端
		client := proto.NewKeyValueServiceClient(conn)
		clients = append(clients, client)
	}

	if len(clients) == 0 {
		return nil, fmt.Errorf("no available servers")
	}

	return &Client{
		clients: clients,
		index:   0,
	}, nil
}

// nextClient 获取下一个客户端（轮询）
func (c *Client) nextClient() proto.KeyValueServiceClient {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	client := c.clients[c.index]
	c.index = (c.index + 1) % len(c.clients)
	return client
}

// Set 设置键值对
func (c *Client) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	client := c.nextClient()

	req := &proto.SetRequest{
		Key:   key,
		Value: value,
	}

	_, err := client.Set(ctx, req)
	if err != nil {
		// 尝试使用下一个客户端
		return c.retrySet(ctx, key, value, ttl)
	}

	return nil
}

// retrySet 重试设置键值对
func (c *Client) retrySet(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	for i := 0; i < len(c.clients); i++ {
		client := c.nextClient()

		req := &proto.SetRequest{
			Key:   key,
			Value: value,
		}

		_, err := client.Set(ctx, req)
		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("all servers failed")
}

// Get 获取键值对
func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	client := c.nextClient()

	req := &proto.GetRequest{
		Key: key,
	}

	resp, err := client.Get(ctx, req)
	if err != nil {
		// 尝试使用下一个客户端
		return c.retryGet(ctx, key)
	}

	if !resp.Found {
		return nil, fmt.Errorf("key not found")
	}

	return resp.Value, nil
}

// retryGet 重试获取键值对
func (c *Client) retryGet(ctx context.Context, key string) ([]byte, error) {
	for i := 0; i < len(c.clients); i++ {
		client := c.nextClient()

		req := &proto.GetRequest{
			Key: key,
		}

		resp, err := client.Get(ctx, req)
		if err == nil {
			if resp.Found {
				return resp.Value, nil
			}
			return nil, fmt.Errorf("key not found")
		}
	}

	return nil, fmt.Errorf("all servers failed")
}

// Delete 删除键值对
func (c *Client) Delete(ctx context.Context, key string) error {
	client := c.nextClient()

	req := &proto.DeleteRequest{
		Key: key,
	}

	_, err := client.Delete(ctx, req)
	if err != nil {
		// 尝试使用下一个客户端
		return c.retryDelete(ctx, key)
	}

	return nil
}

// retryDelete 重试删除键值对
func (c *Client) retryDelete(ctx context.Context, key string) error {
	for i := 0; i < len(c.clients); i++ {
		client := c.nextClient()

		req := &proto.DeleteRequest{
			Key: key,
		}

		_, err := client.Delete(ctx, req)
		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("all servers failed")
}
