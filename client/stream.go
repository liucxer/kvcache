package client

import (
	"context"
	"fmt"
	"io"
	"log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"kvcache/proto"
)

func (c *Client) getStreamClient(addr string) (proto.KVStreamServiceClient, error) {
	conn, err := c.getConnection(addr)
	if err != nil {
		return nil, err
	}
	return proto.NewKVStreamServiceClient(conn), nil
}

// SetStream 流式写入：value 按 1MB 分帧发送，与服务端落盘处理流水线重叠，
// 大 value（如 4MB 数据块）的端到端延迟显著低于 unary Set。
// 路由与索引维护逻辑和 Set 完全一致。
func (c *Client) SetStream(ctx context.Context, key string, value []byte) error {
	inst, err := c.picker.Pick()
	if err != nil {
		return err
	}

	streamClient, err := c.getStreamClient(inst.Addr)
	if err != nil {
		return err
	}

	stream, err := streamClient.SetStream(ctx)
	if err != nil {
		return fmt.Errorf("failed to open set stream on instance %s: %v", inst.Name, err)
	}

	for off := 0; ; off += proto.StreamChunkSize {
		end := off + proto.StreamChunkSize
		if end > len(value) {
			end = len(value)
		}
		chunk := &proto.SetChunk{Data: value[off:end]}
		if off == 0 {
			chunk.Key = []byte(key)
		}
		if err := stream.Send(chunk); err != nil {
			return fmt.Errorf("failed to send chunk to instance %s: %v", inst.Name, err)
		}
		if end == len(value) {
			break
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("failed to close set stream on instance %s: %v", inst.Name, err)
	}
	if !resp.Success {
		return fmt.Errorf("set stream rejected by instance %s: %s", inst.Name, resp.Error)
	}

	if err := c.index.Put(ctx, key, inst.Name); err != nil {
		log.Printf("WARNING: Failed to update index for key %s: %v", key, err)
	}

	c.cache.Put(key, inst.Name)
	return nil
}

// GetStream 流式读取：路由逻辑与 Get 一致，value 分帧接收后拼接。
func (c *Client) GetStream(ctx context.Context, key string) ([]byte, error) {
	if instName, ok := c.cache.Get(key); ok {
		if val, err := c.getFromInstanceStream(ctx, instName, key); err == nil {
			return val, nil
		} else if err == ErrKeyNotFound || isInstanceOffline(err) {
			c.cache.Delete(key)
		} else {
			return nil, err
		}
	}

	indexData, err := c.index.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if indexData == nil {
		return nil, ErrKeyNotFound
	}

	c.cache.Put(key, indexData.Instance)

	val, err := c.getFromInstanceStream(ctx, indexData.Instance, key)
	if err != nil {
		if err == ErrKeyNotFound || isInstanceOffline(err) {
			c.cache.Delete(key)
		}
		return nil, err
	}

	return val, nil
}

func (c *Client) getFromInstanceStream(ctx context.Context, instName, key string) ([]byte, error) {
	insts := c.registry.GetActiveInstances()
	inst, ok := insts[instName]
	if !ok {
		return nil, fmt.Errorf("instance %s is offline", instName)
	}

	streamClient, err := c.getStreamClient(inst.Addr)
	if err != nil {
		return nil, err
	}

	stream, err := streamClient.GetStream(ctx, &proto.GetRequest{Key: []byte(key)})
	if err != nil {
		return nil, err
	}

	var buf []byte
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return buf, nil
		}
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return nil, ErrKeyNotFound
			}
			return nil, err
		}
		buf = append(buf, chunk.Data...)
	}
}
