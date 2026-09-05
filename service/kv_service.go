package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"kvcache/config"
	"kvcache/storage"
)

// KVService 键值存储服务
type KVService struct {
	storage storage.Storage
	config  *config.Config
	metrics *Metrics
	cache   sync.Map // 内存缓存，使用sync.Map保证线程安全
	// cacheCount 近似跟踪 cache 中的 key 数（配合 config.Cache.MaxKeys 防无界增长）。
	// 并发下允许少量误差，缓存场景可接受。
	cacheCount int64
}

// NewKVService 创建新的键值存储服务实例
func NewKVService(storage storage.Storage, config *config.Config) *KVService {
	metrics := NewMetrics()
	return &KVService{
		storage: storage,
		config:  config,
		metrics: metrics,
	}
}

// cacheStore 写入内存缓存，带条数上限（cacheCount 为近似计数，并发下允许少量误差）。
// 缓存满时新 key 不再进入（不做淘汰，简单防 OOM）；已存在的 key 正常覆盖更新。
func (s *KVService) cacheStore(key string, value []byte) {
	maxKeys := int64(s.config.Cache.MaxKeys)
	if maxKeys > 0 && atomic.LoadInt64(&s.cacheCount) >= maxKeys {
		if _, ok := s.cache.Load(key); !ok {
			return
		}
	}
	if _, loaded := s.cache.LoadOrStore(key, value); !loaded {
		atomic.AddInt64(&s.cacheCount, 1)
	} else {
		s.cache.Store(key, value)
	}
}

// cacheDelete 从内存缓存删除并维护计数
func (s *KVService) cacheDelete(key string) {
	if _, ok := s.cache.LoadAndDelete(key); ok {
		atomic.AddInt64(&s.cacheCount, -1)
	}
}

// Set 设置键值对
func (s *KVService) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		s.metrics.SetLatency.WithLabelValues("kv").Observe(time.Since(start).Seconds())
	}()

	if key == "" {
		s.metrics.SetErrors.WithLabelValues("empty_key").Inc()
		return errors.New("empty key")
	}

	err := s.storage.Set([]byte(key), value)
	if err != nil {
		s.metrics.SetErrors.WithLabelValues(err.Error()).Inc()
		return err
	}

	// 检查是否需要写入缓存
	if s.config.Cache.Enabled && len(value) < s.config.Cache.SizeThreshold {
		s.cacheStore(key, value)
	}

	s.metrics.Sets.Inc()
	s.metrics.Keys.Inc()
	return nil
}

// Get 获取值
func (s *KVService) Get(ctx context.Context, key string) ([]byte, error) {
	start := time.Now()
	defer func() {
		s.metrics.GetLatency.WithLabelValues("kv").Observe(time.Since(start).Seconds())
	}()

	if key == "" {
		s.metrics.GetErrors.WithLabelValues("empty_key").Inc()
		return nil, errors.New("empty key")
	}

	// 优先从缓存中查询
	if s.config.Cache.Enabled {
		if cachedValue, ok := s.cache.Load(key); ok {
			s.metrics.Gets.Inc()
			return cachedValue.([]byte), nil
		}
	}

	value, found, err := s.storage.Get([]byte(key))
	if err != nil {
		s.metrics.GetErrors.WithLabelValues(err.Error()).Inc()
		return nil, err
	}

	if !found {
		s.metrics.GetErrors.WithLabelValues("not_found").Inc()
		return nil, errors.New("key not found")
	}

	// 如果值小于缓存阈值，并且缓存未命中，则将值写入缓存
	if s.config.Cache.Enabled && len(value) < s.config.Cache.SizeThreshold {
		s.cacheStore(key, value)
	}

	s.metrics.Gets.Inc()
	return value, nil
}

// Delete 删除键值对
func (s *KVService) Delete(ctx context.Context, key string) error {
	start := time.Now()
	defer func() {
		s.metrics.DeleteLatency.WithLabelValues("kv").Observe(time.Since(start).Seconds())
	}()

	if key == "" {
		s.metrics.DeleteErrors.WithLabelValues("empty_key").Inc()
		return errors.New("empty key")
	}

	err := s.storage.Delete([]byte(key))
	if err != nil {
		s.metrics.DeleteErrors.WithLabelValues(err.Error()).Inc()
		return err
	}

	// 从缓存中删除
	if s.config.Cache.Enabled {
		s.cacheDelete(key)
	}

	s.metrics.Deletes.Inc()
	s.metrics.Keys.Dec()
	return nil
}

// Scan 扫描键值对
func (s *KVService) Scan(ctx context.Context, prefix string, limit int) (map[string][]byte, error) {
	start := time.Now()
	defer func() {
		s.metrics.ScanLatency.WithLabelValues("kv").Observe(time.Since(start).Seconds())
	}()

	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	results, err := s.storage.ScanWithValues([]byte(prefix), limit)
	if err != nil {
		s.metrics.ScanErrors.WithLabelValues(err.Error()).Inc()
		return nil, err
	}

	s.metrics.Scans.Inc()
	return results, nil
}

// ScanKeys 扫描键（只返回键，不返回值）
func (s *KVService) ScanKeys(ctx context.Context, prefix string, limit int) ([]string, error) {
	start := time.Now()
	defer func() {
		s.metrics.ScanLatency.WithLabelValues("kv").Observe(time.Since(start).Seconds())
	}()

	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	keys, err := s.storage.Scan([]byte(prefix), limit)
	if err != nil {
		s.metrics.ScanErrors.WithLabelValues(err.Error()).Inc()
		return nil, err
	}

	// 转换 [][]byte 为 []string
	result := make([]string, len(keys))
	for i, key := range keys {
		result[i] = string(key)
	}

	s.metrics.Scans.Inc()
	return result, nil
}

// MSet 批量设置键值对
func (s *KVService) MSet(ctx context.Context, kvs map[string][]byte, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		s.metrics.MSetLatency.WithLabelValues("kv").Observe(time.Since(start).Seconds())
	}()

	if len(kvs) == 0 {
		s.metrics.MSetErrors.WithLabelValues("empty_kvs").Inc()
		return errors.New("empty key-value pairs")
	}

	err := s.storage.MSet(kvs)
	if err != nil {
		s.metrics.MSetErrors.WithLabelValues(err.Error()).Inc()
		return err
	}

	// 批量写入缓存
	if s.config.Cache.Enabled {
		for key, value := range kvs {
			if len(value) < s.config.Cache.SizeThreshold {
				s.cacheStore(key, value)
			}
		}
	}

	s.metrics.MSets.Inc()
	s.metrics.Keys.Add(float64(len(kvs)))
	return nil
}

// MGet 批量获取值
func (s *KVService) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	start := time.Now()
	defer func() {
		s.metrics.MGetLatency.WithLabelValues("kv").Observe(time.Since(start).Seconds())
	}()

	if len(keys) == 0 {
		s.metrics.MGetErrors.WithLabelValues("empty_keys").Inc()
		return nil, errors.New("empty keys")
	}

	results := make(map[string][]byte)
	missedKeys := make([]string, 0)

	// 优先从缓存中查询
	if s.config.Cache.Enabled {
		for _, key := range keys {
			if cachedValue, ok := s.cache.Load(key); ok {
				results[key] = cachedValue.([]byte)
			} else {
				missedKeys = append(missedKeys, key)
			}
		}
	} else {
		missedKeys = keys
	}

	// 从存储中查询缓存未命中的key
	if len(missedKeys) > 0 {
		// 转换missedKeys为[][]byte
		byteKeys := make([][]byte, len(missedKeys))
		for i, key := range missedKeys {
			byteKeys[i] = []byte(key)
		}

		storageResults, err := s.storage.MGet(byteKeys)
		if err != nil {
			s.metrics.MGetErrors.WithLabelValues(err.Error()).Inc()
			return nil, err
		}

		// 合并结果并写入缓存
		for key, value := range storageResults {
			results[key] = value
			// 如果值小于缓存阈值，将其写入缓存
			if s.config.Cache.Enabled && len(value) < s.config.Cache.SizeThreshold {
				s.cacheStore(key, value)
			}
		}
	}

	s.metrics.MGets.Inc()
	return results, nil
}

// MDelete 批量删除键值对
func (s *KVService) MDelete(ctx context.Context, keys []string) error {
	start := time.Now()
	defer func() {
		s.metrics.MDeleteLatency.WithLabelValues("kv").Observe(time.Since(start).Seconds())
	}()

	if len(keys) == 0 {
		s.metrics.MDeleteErrors.WithLabelValues("empty_keys").Inc()
		return errors.New("empty keys")
	}

	// 转换keys为[][]byte
	byteKeys := make([][]byte, len(keys))
	for i, key := range keys {
		byteKeys[i] = []byte(key)
	}

	err := s.storage.MDelete(byteKeys)
	if err != nil {
		s.metrics.MDeleteErrors.WithLabelValues(err.Error()).Inc()
		return err
	}

	// 批量从缓存中删除
	if s.config.Cache.Enabled {
		for _, key := range keys {
			s.cacheDelete(key)
		}
	}

	s.metrics.MDeletes.Inc()
	s.metrics.Keys.Sub(float64(len(keys)))
	return nil
}

// GetConfig 获取配置
func (s *KVService) GetConfig(ctx context.Context) (*config.Config, error) {
	start := time.Now()
	defer func() {
		s.metrics.GetLatency.WithLabelValues("config").Observe(time.Since(start).Seconds())
	}()

	return s.config, nil
}

// UpdateConfig 更新配置
func (s *KVService) UpdateConfig(ctx context.Context, newConfig *config.Config) error {
	start := time.Now()
	defer func() {
		s.metrics.SetLatency.WithLabelValues("config").Observe(time.Since(start).Seconds())
	}()

	err := s.storage.UpdateConfig(newConfig)
	if err != nil {
		s.metrics.SetErrors.WithLabelValues("config_update").Inc()
		return err
	}

	// 更新内存中的配置
	s.config = newConfig

	s.metrics.ConfigUpdates.Inc()
	return nil
}

// HealthCheck 健康检查
func (s *KVService) HealthCheck(ctx context.Context) error {
	start := time.Now()
	defer func() {
		s.metrics.HealthCheckLatency.Observe(time.Since(start).Seconds())
	}()

	s.metrics.HealthChecks.Inc()
	// 直接返回nil，表示服务正常
	return nil
}
