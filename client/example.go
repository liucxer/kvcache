package client

import (
	"context"
	"log"
	"time"
)

// Example 使用示例
func Example() {
	// 假设我们有6个kvcache实例运行在不同的端口上
	// 实际部署中，这些地址应该从配置文件或服务发现中获取
	serverAddrs := []string{
		"localhost:33000", // 节点1实例1
		"localhost:33002", // 节点1实例2
		"localhost:33004", // 节点2实例1
		"localhost:33006", // 节点2实例2
		"localhost:33008", // 节点3实例1
		"localhost:33010", // 节点3实例2
	}

	// 创建客户端
	client, err := NewClient(serverAddrs)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// 创建上下文
	ctx := context.Background()

	// 设置键值对
	key := "test-key"
	value := []byte("test-value")
	err = client.Set(ctx, key, value, 0)
	if err != nil {
		log.Printf("Failed to set key: %v", err)
	} else {
		log.Printf("Set key %s successfully", key)
	}

	// 获取键值对
	retrievedValue, err := client.Get(ctx, key)
	if err != nil {
		log.Printf("Failed to get key: %v", err)
	} else {
		log.Printf("Get key %s: %s", key, retrievedValue)
	}

	// 删除键值对
	err = client.Delete(ctx, key)
	if err != nil {
		log.Printf("Failed to delete key: %v", err)
	} else {
		log.Printf("Delete key %s successfully", key)
	}

	// 尝试获取已删除的键
	_, err = client.Get(ctx, key)
	if err != nil {
		log.Printf("Expected error when getting deleted key: %v", err)
	}
}
