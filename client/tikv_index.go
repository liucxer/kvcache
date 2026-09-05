package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/tikv/client-go/v2/rawkv"
)

const (
	indexKeyPrefix = "/kvcache/index/"
)

type IndexData struct {
	Instance  string `json:"instance"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type IndexManager struct {
	tikvClient *rawkv.Client
}

func NewIndexManager(tikvClient *rawkv.Client) *IndexManager {
	return &IndexManager{
		tikvClient: tikvClient,
	}
}

// Put 写入索引。单次 rawkv Put 完成，不做读-改-写：
// 旧实现为先 Get 保留 CreatedAt 再 Put，每个 Set 两次 TiKV 往返，
// 且所有索引 key 同前缀落在单个 region，高并发下成为串行瓶颈。
// CreatedAt 没有实际消费者（与 BatchPut 行为对齐，均取当前时间）。
func (im *IndexManager) Put(ctx context.Context, key, instance string) error {
	now := time.Now().Unix()

	data := &IndexData{
		Instance:  instance,
		CreatedAt: now,
		UpdatedAt: now,
	}

	value, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal index data: %v", err)
	}

	tikvKey := []byte(indexKeyPrefix + key)
	if err := im.tikvClient.Put(ctx, tikvKey, value); err != nil {
		return fmt.Errorf("failed to put index: %v", err)
	}

	return nil
}

func (im *IndexManager) Get(ctx context.Context, key string) (*IndexData, error) {
	tikvKey := []byte(indexKeyPrefix + key)
	value, err := im.tikvClient.Get(ctx, tikvKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get index: %v", err)
	}
	if value == nil || len(value) == 0 {
		return nil, nil
	}

	var data IndexData
	if err := json.Unmarshal(value, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal index data: %v", err)
	}

	return &data, nil
}

func (im *IndexManager) Delete(ctx context.Context, key string) error {
	tikvKey := []byte(indexKeyPrefix + key)
	if err := im.tikvClient.Delete(ctx, tikvKey); err != nil {
		return fmt.Errorf("failed to delete index: %v", err)
	}
	return nil
}

// BatchPut 批量写入索引。注意：与单个 Put 不同，这里不做读-改-写保留 CreatedAt
// （批量路径要的是吞吐，CreatedAt 统一取当前时间），一次 rawkv BatchPut 完成。
func (im *IndexManager) BatchPut(ctx context.Context, kvs map[string]string) map[string]error {
	now := time.Now().Unix()
	keys := make([][]byte, 0, len(kvs))
	values := make([][]byte, 0, len(kvs))

	for key, instance := range kvs {
		data := &IndexData{
			Instance:  instance,
			CreatedAt: now,
			UpdatedAt: now,
		}
		value, err := json.Marshal(data)
		if err != nil {
			return map[string]error{key: fmt.Errorf("failed to marshal index data: %v", err)}
		}
		keys = append(keys, []byte(indexKeyPrefix+key))
		values = append(values, value)
	}

	if err := im.tikvClient.BatchPut(ctx, keys, values); err != nil {
		errors := make(map[string]error, len(kvs))
		for key := range kvs {
			errors[key] = fmt.Errorf("failed to batch put index: %v", err)
		}
		return errors
	}

	return map[string]error{}
}

// BatchGet 批量查询索引，底层一次 rawkv BatchGet（返回的 values 与 keys 按下标对应，
// 不存在的 key 对应 nil value）。
func (im *IndexManager) BatchGet(ctx context.Context, keys []string) (map[string]*IndexData, map[string]error) {
	results := make(map[string]*IndexData)

	tikvKeys := make([][]byte, len(keys))
	for i, key := range keys {
		tikvKeys[i] = []byte(indexKeyPrefix + key)
	}

	values, err := im.tikvClient.BatchGet(ctx, tikvKeys)
	if err != nil {
		errors := make(map[string]error, len(keys))
		for _, key := range keys {
			errors[key] = fmt.Errorf("failed to batch get index: %v", err)
		}
		return results, errors
	}

	for i, value := range values {
		if len(value) == 0 {
			continue
		}
		var data IndexData
		if err := json.Unmarshal(value, &data); err != nil {
			log.Printf("WARNING: Failed to unmarshal index data for key %s: %v", keys[i], err)
			continue
		}
		results[keys[i]] = &data
	}

	return results, map[string]error{}
}

// BatchDelete 批量删除索引，底层一次 rawkv BatchDelete。
func (im *IndexManager) BatchDelete(ctx context.Context, keys []string) map[string]error {
	tikvKeys := make([][]byte, len(keys))
	for i, key := range keys {
		tikvKeys[i] = []byte(indexKeyPrefix + key)
	}

	if err := im.tikvClient.BatchDelete(ctx, tikvKeys); err != nil {
		errors := make(map[string]error, len(keys))
		for _, key := range keys {
			errors[key] = fmt.Errorf("failed to batch delete index: %v", err)
		}
		return errors
	}

	return map[string]error{}
}
