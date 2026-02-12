package service

import (
	"context"
	"math/rand"
	"os"
	"testing"

	"kvcache/config"
	"kvcache/storage"
)

// 性能测试：开启缓存 - 单线程set操作
func BenchmarkSetWithCache(b *testing.B) {
	// 初始化配置
	cfg := config.DefaultConfig()
	// 确保开启缓存
	cfg.Cache.Enabled = true

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建KV服务实例
	service := NewKVService(store, cfg)

	// 重置计时器
	b.ResetTimer()

	// 执行set操作
	for i := 0; i < b.N; i++ {
		key := "benchmark-key-" + randomString(10)
		value := []byte("benchmark-value-" + randomString(50))
		err := service.Set(context.Background(), key, value, 0)
		if err != nil {
			b.Fatalf("Failed to set: %v", err)
		}
	}
}

// 性能测试：未开启缓存 - 单线程set操作
func BenchmarkSetWithoutCache(b *testing.B) {
	// 初始化配置
	cfg := config.DefaultConfig()
	// 确保关闭缓存
	cfg.Cache.Enabled = false

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建KV服务实例
	service := NewKVService(store, cfg)

	// 重置计时器
	b.ResetTimer()

	// 执行set操作
	for i := 0; i < b.N; i++ {
		key := "benchmark-key-" + randomString(10)
		value := []byte("benchmark-value-" + randomString(50))
		err := service.Set(context.Background(), key, value, 0)
		if err != nil {
			b.Fatalf("Failed to set: %v", err)
		}
	}
}

// 性能测试：开启缓存 - 单线程get操作
func BenchmarkGetWithCache(b *testing.B) {
	// 初始化配置
	cfg := config.DefaultConfig()
	// 确保开启缓存
	cfg.Cache.Enabled = true

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建KV服务实例
	service := NewKVService(store, cfg)

	// 预先设置一些键值对
	keys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		key := "benchmark-key-" + randomString(10)
		keys[i] = key
		value := []byte("benchmark-value-" + randomString(50))
		err := service.Set(context.Background(), key, value, 0)
		if err != nil {
			b.Fatalf("Failed to set: %v", err)
		}
	}

	// 重置计时器
	b.ResetTimer()

	// 执行get操作
	for i := 0; i < b.N; i++ {
		key := keys[rand.Intn(len(keys))]
		_, err := service.Get(context.Background(), key)
		if err != nil {
			b.Fatalf("Failed to get: %v", err)
		}
	}
}

// 性能测试：未开启缓存 - 单线程get操作
func BenchmarkGetWithoutCache(b *testing.B) {
	// 初始化配置
	cfg := config.DefaultConfig()
	// 确保关闭缓存
	cfg.Cache.Enabled = false

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建KV服务实例
	service := NewKVService(store, cfg)

	// 预先设置一些键值对
	keys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		key := "benchmark-key-" + randomString(10)
		keys[i] = key
		value := []byte("benchmark-value-" + randomString(50))
		err := service.Set(context.Background(), key, value, 0)
		if err != nil {
			b.Fatalf("Failed to set: %v", err)
		}
	}

	// 重置计时器
	b.ResetTimer()

	// 执行get操作
	for i := 0; i < b.N; i++ {
		key := keys[rand.Intn(len(keys))]
		_, err := service.Get(context.Background(), key)
		if err != nil {
			b.Fatalf("Failed to get: %v", err)
		}
	}
}

// 性能测试：开启缓存 - 并发set操作
func BenchmarkSetConcurrentWithCache(b *testing.B) {
	// 初始化配置
	cfg := config.DefaultConfig()
	// 确保开启缓存
	cfg.Cache.Enabled = true

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建KV服务实例
	service := NewKVService(store, cfg)

	// 重置计时器
	b.ResetTimer()

	// 执行并发set操作
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := "benchmark-key-" + randomString(10)
			value := []byte("benchmark-value-" + randomString(50))
			err := service.Set(context.Background(), key, value, 0)
			if err != nil {
				b.Fatalf("Failed to set: %v", err)
			}
		}
	})
}

// 性能测试：未开启缓存 - 并发set操作
func BenchmarkSetConcurrentWithoutCache(b *testing.B) {
	// 初始化配置
	cfg := config.DefaultConfig()
	// 确保关闭缓存
	cfg.Cache.Enabled = false

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建KV服务实例
	service := NewKVService(store, cfg)

	// 重置计时器
	b.ResetTimer()

	// 执行并发set操作
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := "benchmark-key-" + randomString(10)
			value := []byte("benchmark-value-" + randomString(50))
			err := service.Set(context.Background(), key, value, 0)
			if err != nil {
				b.Fatalf("Failed to set: %v", err)
			}
		}
	})
}

// 性能测试：开启缓存 - 并发get操作
func BenchmarkGetConcurrentWithCache(b *testing.B) {
	// 初始化配置
	cfg := config.DefaultConfig()
	// 确保开启缓存
	cfg.Cache.Enabled = true

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建KV服务实例
	service := NewKVService(store, cfg)

	// 预先设置一些键值对
	keys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		key := "benchmark-key-" + randomString(10)
		keys[i] = key
		value := []byte("benchmark-value-" + randomString(50))
		err := service.Set(context.Background(), key, value, 0)
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
			_, err := service.Get(context.Background(), key)
			if err != nil {
				b.Fatalf("Failed to get: %v", err)
			}
		}
	})
}

// 性能测试：未开启缓存 - 并发get操作
func BenchmarkGetConcurrentWithoutCache(b *testing.B) {
	// 初始化配置
	cfg := config.DefaultConfig()
	// 确保关闭缓存
	cfg.Cache.Enabled = false

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建KV服务实例
	service := NewKVService(store, cfg)

	// 预先设置一些键值对
	keys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		key := "benchmark-key-" + randomString(10)
		keys[i] = key
		value := []byte("benchmark-value-" + randomString(50))
		err := service.Set(context.Background(), key, value, 0)
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
			_, err := service.Get(context.Background(), key)
			if err != nil {
				b.Fatalf("Failed to get: %v", err)
			}
		}
	})
}

// 性能测试：开启缓存 - 混合操作
func BenchmarkMixedOperationsWithCache(b *testing.B) {
	// 初始化配置
	cfg := config.DefaultConfig()
	// 确保开启缓存
	cfg.Cache.Enabled = true

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建KV服务实例
	service := NewKVService(store, cfg)

	// 预先设置一些键值对
	keys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		key := "benchmark-key-" + randomString(10)
		keys[i] = key
		value := []byte("benchmark-value-" + randomString(50))
		err := service.Set(context.Background(), key, value, 0)
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
				_, err := service.Get(context.Background(), key)
				if err != nil {
					b.Fatalf("Failed to get: %v", err)
				}
			} else {
				// 20%的概率执行set操作
				key := "benchmark-key-" + randomString(10)
				value := []byte("benchmark-value-" + randomString(50))
				err := service.Set(context.Background(), key, value, 0)
				if err != nil {
					b.Fatalf("Failed to set: %v", err)
				}
			}
		}
	})
}

// 性能测试：未开启缓存 - 混合操作
func BenchmarkMixedOperationsWithoutCache(b *testing.B) {
	// 初始化配置
	cfg := config.DefaultConfig()
	// 确保关闭缓存
	cfg.Cache.Enabled = false

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建KV服务实例
	service := NewKVService(store, cfg)

	// 预先设置一些键值对
	keys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		key := "benchmark-key-" + randomString(10)
		keys[i] = key
		value := []byte("benchmark-value-" + randomString(50))
		err := service.Set(context.Background(), key, value, 0)
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
				_, err := service.Get(context.Background(), key)
				if err != nil {
					b.Fatalf("Failed to get: %v", err)
				}
			} else {
				// 20%的概率执行set操作
				key := "benchmark-key-" + randomString(10)
				value := []byte("benchmark-value-" + randomString(50))
				err := service.Set(context.Background(), key, value, 0)
				if err != nil {
					b.Fatalf("Failed to set: %v", err)
				}
			}
		}
	})
}
