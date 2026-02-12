package api_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"kvcache/proto"
)

// 初始化随机数生成器
func init() {
	rand.Seed(time.Now().UnixNano())
}

// 生成随机字符串
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

// 性能测试：gRPC客户端单线程set操作
func BenchmarkGRPCSet(b *testing.B) {
	// 重置计时器
	b.ResetTimer()

	// 执行set操作
	for i := 0; i < b.N; i++ {
		key := []byte("grpc-benchmark-key-" + randomString(10))
		value := []byte("grpc-benchmark-value-" + randomString(50))

		// 创建请求
		req := &proto.SetRequest{
			Key:   key,
			Value: value,
		}

		// 发送请求
		_, err := grpcClient.Set(context.Background(), req)
		if err != nil {
			b.Fatalf("Failed to set: %v", err)
		}
	}
}

// 性能测试：gRPC客户端单线程get操作
func BenchmarkGRPCGet(b *testing.B) {
	// 预热阶段：预先设置一些键值对
	keys := make([][]byte, 1000)
	for i := 0; i < 1000; i++ {
		key := []byte("grpc-benchmark-key-" + randomString(10))
		keys[i] = key
		value := []byte("grpc-benchmark-value-" + randomString(50))

		// 创建请求
		req := &proto.SetRequest{
			Key:   key,
			Value: value,
		}

		// 发送请求
		_, err := grpcClient.Set(context.Background(), req)
		if err != nil {
			b.Fatalf("Failed to set: %v", err)
		}
	}

	// 重置计时器
	b.ResetTimer()

	// 执行get操作
	for i := 0; i < b.N; i++ {
		key := keys[rand.Intn(len(keys))]

		// 创建请求
		req := &proto.GetRequest{
			Key: key,
		}

		// 发送请求
		_, err := grpcClient.Get(context.Background(), req)
		if err != nil {
			b.Fatalf("Failed to get: %v", err)
		}
	}
}

// 性能测试：gRPC客户端并发set操作
func BenchmarkGRPCSetConcurrent(b *testing.B) {
	// 重置计时器
	b.ResetTimer()

	// 执行并发set操作
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := []byte("grpc-benchmark-key-" + randomString(10))
			value := []byte("grpc-benchmark-value-" + randomString(50))

			// 创建请求
			req := &proto.SetRequest{
				Key:   key,
				Value: value,
			}

			// 发送请求
			_, err := grpcClient.Set(context.Background(), req)
			if err != nil {
				b.Fatalf("Failed to set: %v", err)
			}
		}
	})
}

// 性能测试：gRPC客户端并发get操作
func BenchmarkGRPCGetConcurrent(b *testing.B) {
	// 预热阶段：预先设置一些键值对
	keys := make([][]byte, 1000)
	for i := 0; i < 1000; i++ {
		key := []byte("grpc-benchmark-key-" + randomString(10))
		keys[i] = key
		value := []byte("grpc-benchmark-value-" + randomString(50))

		// 创建请求
		req := &proto.SetRequest{
			Key:   key,
			Value: value,
		}

		// 发送请求
		_, err := grpcClient.Set(context.Background(), req)
		if err != nil {
			b.Fatalf("Failed to set: %v", err)
		}
	}

	// 重置计时器
	b.ResetTimer()

	// 执行并发get操作
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := keys[rand.Intn(len(keys))]

			// 创建请求
			req := &proto.GetRequest{
				Key: key,
			}

			// 发送请求
			_, err := grpcClient.Get(context.Background(), req)
			if err != nil {
				b.Fatalf("Failed to get: %v", err)
			}
		}
	})
}

// 性能测试：gRPC客户端混合操作（set和get）
func BenchmarkGRPCMixedOperations(b *testing.B) {
	// 预热阶段：预先设置一些键值对
	keys := make([][]byte, 1000)
	for i := 0; i < 1000; i++ {
		key := []byte("grpc-benchmark-key-" + randomString(10))
		keys[i] = key
		value := []byte("grpc-benchmark-value-" + randomString(50))

		// 创建请求
		req := &proto.SetRequest{
			Key:   key,
			Value: value,
		}

		// 发送请求
		_, err := grpcClient.Set(context.Background(), req)
		if err != nil {
			b.Fatalf("Failed to set: %v", err)
		}
	}

	// 重置计时器
	b.ResetTimer()

	// 执行混合操作
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// 随机选择操作类型
			if rand.Float64() < 0.8 {
				// 80%的概率执行get操作
				key := keys[rand.Intn(len(keys))]

				// 创建请求
				req := &proto.GetRequest{
					Key: key,
				}

				// 发送请求
				_, err := grpcClient.Get(context.Background(), req)
				if err != nil {
					b.Fatalf("Failed to get: %v", err)
				}
			} else {
				// 20%的概率执行set操作
				key := []byte("grpc-benchmark-key-" + randomString(10))
				value := []byte("grpc-benchmark-value-" + randomString(50))

				// 创建请求
				req := &proto.SetRequest{
					Key:   key,
					Value: value,
				}

				// 发送请求
				_, err := grpcClient.Set(context.Background(), req)
				if err != nil {
					b.Fatalf("Failed to set: %v", err)
				}
			}
		}
	})
}
