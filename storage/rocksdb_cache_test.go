package storage

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"kvcache/config"
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

// 性能测试：不同RocksDB Block Cache大小的对比
func BenchmarkRocksDBWithDifferentCacheSizes(b *testing.B) {
	// 测试不同的Block Cache大小（MB），包括关闭Block Cache的情况（0MB）
	cacheSizes := []int{0, 8, 16, 32, 64, 128, 256}

	for _, cacheSize := range cacheSizes {
		b.Run(fmt.Sprintf("CacheSize_%dMB", cacheSize), func(b *testing.B) {
			// 初始化配置
			cfg := config.DefaultConfig()
			// 关闭内存缓存
			cfg.Cache.Enabled = false
			// 设置RocksDB Block Cache大小
			cfg.RocksDB.BlockCacheSize = cacheSize

			// 删除现有的数据目录，确保测试环境干净
			os.RemoveAll(cfg.RocksDB.Path)
			os.RemoveAll(cfg.Value.DiskPath)

			// 创建存储实例
			store, err := NewStorage(cfg)
			if err != nil {
				b.Fatalf("Failed to create storage: %v", err)
			}
			defer store.Stop()

			// 预先设置一些键值对（10万级别）
			keys := make([]string, 100000)
			for i := 0; i < 100000; i++ {
				key := "benchmark-key-" + randomString(10)
				keys[i] = key
				value := []byte("benchmark-value-" + randomString(50))
				err := store.Set([]byte(key), value)
				if err != nil {
					b.Fatalf("Failed to set: %v", err)
				}
			}

			// 重置计时器
			b.ResetTimer()

			// 执行混合操作（80% Get + 20% Set）
			for i := 0; i < b.N; i++ {
				if rand.Float64() < 0.8 {
					// 80%的概率执行get操作
					key := keys[rand.Intn(len(keys))]
					_, _, err := store.Get([]byte(key))
					if err != nil {
						b.Fatalf("Failed to get: %v", err)
					}
				} else {
					// 20%的概率执行set操作
					key := "benchmark-key-" + randomString(10)
					value := []byte("benchmark-value-" + randomString(50))
					err := store.Set([]byte(key), value)
					if err != nil {
						b.Fatalf("Failed to set: %v", err)
					}
				}
			}
		})
	}
}

// 性能测试：不同RocksDB Block Cache大小的并发性能对比
func BenchmarkRocksDBConcurrentWithDifferentCacheSizes(b *testing.B) {
	// 测试不同的Block Cache大小（MB），包括关闭Block Cache的情况（0MB）
	cacheSizes := []int{0, 8, 16, 32, 64, 128, 256}

	for _, cacheSize := range cacheSizes {
		b.Run(fmt.Sprintf("CacheSize_%dMB", cacheSize), func(b *testing.B) {
			// 初始化配置
			cfg := config.DefaultConfig()
			// 关闭内存缓存
			cfg.Cache.Enabled = false
			// 设置RocksDB Block Cache大小
			cfg.RocksDB.BlockCacheSize = cacheSize

			// 删除现有的数据目录，确保测试环境干净
			os.RemoveAll(cfg.RocksDB.Path)
			os.RemoveAll(cfg.Value.DiskPath)

			// 创建存储实例
			store, err := NewStorage(cfg)
			if err != nil {
				b.Fatalf("Failed to create storage: %v", err)
			}
			defer store.Stop()

			// 预先设置一些键值对（10万级别）
			keys := make([]string, 100000)
			for i := 0; i < 100000; i++ {
				key := "benchmark-key-" + randomString(10)
				keys[i] = key
				value := []byte("benchmark-value-" + randomString(50))
				err := store.Set([]byte(key), value)
				if err != nil {
					b.Fatalf("Failed to set: %v", err)
				}
			}

			// 重置计时器
			b.ResetTimer()

			// 执行并发混合操作
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if rand.Float64() < 0.8 {
						// 80%的概率执行get操作
						key := keys[rand.Intn(len(keys))]
						_, _, err := store.Get([]byte(key))
						if err != nil {
							b.Fatalf("Failed to get: %v", err)
						}
					} else {
						// 20%的概率执行set操作
						key := "benchmark-key-" + randomString(10)
						value := []byte("benchmark-value-" + randomString(50))
						err := store.Set([]byte(key), value)
						if err != nil {
							b.Fatalf("Failed to set: %v", err)
						}
					}
				}
			})
		})
	}
}
