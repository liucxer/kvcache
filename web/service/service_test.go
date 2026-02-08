package service

import (
	"context"
	"os"
	"testing"

	"cachefs/config"
	"cachefs/storage"
)

// TestNewKVService 测试创建KV服务实例
func TestNewKVService(t *testing.T) {
	// 初始化配置
	cfg := config.DefaultConfig()

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建KV服务实例
	service := NewKVService(store, cfg)

	// 测试服务是否成功创建
	if service == nil {
		t.Fatalf("Expected service to be non-nil, got nil")
	}
}

// TestKVServiceSetGet 测试KV服务的设置和获取功能
func TestKVServiceSetGet(t *testing.T) {
	// 初始化配置
	cfg := config.DefaultConfig()

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建KV服务实例
	service := NewKVService(store, cfg)

	// 测试设置值
	testKey := "test-key"
	testValue := []byte("test-value")

	err = service.Set(context.Background(), testKey, testValue, 0)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// 测试获取值
	value, err := service.Get(context.Background(), testKey)
	if err != nil {
		t.Fatalf("Failed to get value: %v", err)
	}

	if string(value) != string(testValue) {
		t.Errorf("Expected value to be '%s', got '%s'", string(testValue), string(value))
	}
}

// TestKVServiceDelete 测试KV服务的删除功能
func TestKVServiceDelete(t *testing.T) {
	// 初始化配置
	cfg := config.DefaultConfig()

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建KV服务实例
	service := NewKVService(store, cfg)

	// 先设置一个值
	testKey := "test-key"
	testValue := []byte("test-value")

	err = service.Set(context.Background(), testKey, testValue, 0)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// 测试删除值
	err = service.Delete(context.Background(), testKey)
	if err != nil {
		t.Fatalf("Failed to delete value: %v", err)
	}

	// 测试获取已删除的值
	value, err := service.Get(context.Background(), testKey)
	if err == nil {
		t.Fatalf("Expected error when getting deleted key, but got nil")
	}

	if value != nil {
		t.Errorf("Expected value to be nil, got '%s'", string(value))
	}
}

// TestKVServiceMSetMGet 测试KV服务的批量设置和批量获取功能
func TestKVServiceMSetMGet(t *testing.T) {
	// 初始化配置
	cfg := config.DefaultConfig()

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建KV服务实例
	service := NewKVService(store, cfg)

	// 测试批量设置值
	testKeyValues := map[string][]byte{
		"batch-key-1": []byte("batch-value-1"),
		"batch-key-2": []byte("batch-value-2"),
	}

	err = service.MSet(context.Background(), testKeyValues, 0)
	if err != nil {
		t.Fatalf("Failed to mset values: %v", err)
	}

	// 测试批量获取值
	testKeys := []string{
		"batch-key-1",
		"batch-key-2",
	}

	results, err := service.MGet(context.Background(), testKeys)
	if err != nil {
		t.Fatalf("Failed to mget values: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	if string(results["batch-key-1"]) != "batch-value-1" {
		t.Errorf("Expected value for batch-key-1 to be 'batch-value-1', got '%s'", string(results["batch-key-1"]))
	}

	if string(results["batch-key-2"]) != "batch-value-2" {
		t.Errorf("Expected value for batch-key-2 to be 'batch-value-2', got '%s'", string(results["batch-key-2"]))
	}
}

// TestKVServiceMDelete 测试KV服务的批量删除功能
func TestKVServiceMDelete(t *testing.T) {
	// 初始化配置
	cfg := config.DefaultConfig()

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建KV服务实例
	service := NewKVService(store, cfg)

	// 先批量设置值
	testKeyValues := map[string][]byte{
		"batch-key-1": []byte("batch-value-1"),
		"batch-key-2": []byte("batch-value-2"),
	}

	err = service.MSet(context.Background(), testKeyValues, 0)
	if err != nil {
		t.Fatalf("Failed to mset values: %v", err)
	}

	// 测试批量删除值
	testKeys := []string{
		"batch-key-1",
		"batch-key-2",
	}

	err = service.MDelete(context.Background(), testKeys)
	if err != nil {
		t.Fatalf("Failed to mdelete values: %v", err)
	}

	// 测试获取已删除的值
	results, err := service.MGet(context.Background(), testKeys)
	if err != nil {
		t.Fatalf("Failed to mget values: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

// TestKVServiceScan 测试KV服务的扫描功能
func TestKVServiceScan(t *testing.T) {
	// 初始化配置
	cfg := config.DefaultConfig()

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建KV服务实例
	service := NewKVService(store, cfg)

	// 先设置几个值
	testKeyValues := map[string][]byte{
		"scan-key-1": []byte("scan-value-1"),
		"scan-key-2": []byte("scan-value-2"),
		"other-key":  []byte("other-value"),
	}

	for key, value := range testKeyValues {
		err = service.Set(context.Background(), key, value, 0)
		if err != nil {
			t.Fatalf("Failed to set value for key '%s': %v", key, err)
		}
	}

	// 测试扫描功能
	results, err := service.Scan(context.Background(), "scan", 100)
	if err != nil {
		t.Fatalf("Failed to scan: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	if string(results["scan-key-1"]) != "scan-value-1" {
		t.Errorf("Expected value for scan-key-1 to be 'scan-value-1', got '%s'", string(results["scan-key-1"]))
	}

	if string(results["scan-key-2"]) != "scan-value-2" {
		t.Errorf("Expected value for scan-key-2 to be 'scan-value-2', got '%s'", string(results["scan-key-2"]))
	}
}

// TestKVServiceConfig 测试KV服务的配置管理功能
func TestKVServiceConfig(t *testing.T) {
	// 初始化配置
	cfg := config.DefaultConfig()

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建KV服务实例
	service := NewKVService(store, cfg)

	// 测试获取配置
	storedCfg, err := service.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if storedCfg == nil {
		t.Fatalf("Expected config to be non-nil, got nil")
	}

	// 测试更新配置
	newCfg := config.DefaultConfig()

	err = service.UpdateConfig(context.Background(), newCfg)
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	// 测试获取更新后的配置
	updatedCfg, err := service.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("Failed to get updated config: %v", err)
	}

	if updatedCfg == nil {
		t.Fatalf("Expected updated config to be non-nil, got nil")
	}
}

// TestKVServiceHealthCheck 测试KV服务的健康检查功能
func TestKVServiceHealthCheck(t *testing.T) {
	// 初始化配置
	cfg := config.DefaultConfig()

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建KV服务实例
	service := NewKVService(store, cfg)

	// 测试健康检查
	err = service.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("Failed to health check: %v", err)
	}
}

// TestKVServiceErrorHandling 测试KV服务的错误处理功能
func TestKVServiceErrorHandling(t *testing.T) {
	// 初始化配置
	cfg := config.DefaultConfig()

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建KV服务实例
	service := NewKVService(store, cfg)

	// 测试空键错误
	err = service.Set(context.Background(), "", []byte("value"), 0)
	if err == nil {
		t.Fatalf("Expected error for empty key, but got nil")
	}

	// 测试获取不存在的键
	_, err = service.Get(context.Background(), "non-existent-key")
	if err == nil {
		t.Fatalf("Expected error for non-existent key, but got nil")
	}

	// 测试删除不存在的键
	err = service.Delete(context.Background(), "non-existent-key")
	// 删除不存在的键应该成功（幂等操作）
	if err != nil {
		t.Fatalf("Expected success for deleting non-existent key, but got error: %v", err)
	}

	// 测试空批量操作
	err = service.MSet(context.Background(), map[string][]byte{}, 0)
	if err == nil {
		t.Fatalf("Expected error for empty MSet, but got nil")
	}

	// 测试空批量获取
	_, err = service.MGet(context.Background(), []string{})
	if err == nil {
		t.Fatalf("Expected error for empty MGet, but got nil")
	}

	// 测试空批量删除
	err = service.MDelete(context.Background(), []string{})
	if err == nil {
		t.Fatalf("Expected error for empty MDelete, but got nil")
	}
}

// TestMetricsInitialization 测试指标初始化功能
func TestMetricsInitialization(t *testing.T) {
	// 测试多次创建指标实例，确保不会重复注册
	// 第一次创建
	metrics1 := NewMetrics()
	if metrics1 == nil {
		t.Fatalf("Expected metrics to be non-nil, got nil")
	}

	// 第二次创建（应该使用相同的注册）
	metrics2 := NewMetrics()
	if metrics2 == nil {
		t.Fatalf("Expected metrics to be non-nil, got nil")
	}

	// 验证指标实例不为nil
	if metrics1.Sets == nil {
		t.Fatalf("Expected Sets counter to be non-nil, got nil")
	}

	if metrics1.Gets == nil {
		t.Fatalf("Expected Gets counter to be non-nil, got nil")
	}

	if metrics1.Deletes == nil {
		t.Fatalf("Expected Deletes counter to be non-nil, got nil")
	}

	if metrics1.SetErrors == nil {
		t.Fatalf("Expected SetErrors counter to be non-nil, got nil")
	}

	if metrics1.GetErrors == nil {
		t.Fatalf("Expected GetErrors counter to be non-nil, got nil")
	}

	if metrics1.DeleteErrors == nil {
		t.Fatalf("Expected DeleteErrors counter to be non-nil, got nil")
	}

	if metrics1.SetLatency == nil {
		t.Fatalf("Expected SetLatency histogram to be non-nil, got nil")
	}

	if metrics1.GetLatency == nil {
		t.Fatalf("Expected GetLatency histogram to be non-nil, got nil")
	}

	if metrics1.DeleteLatency == nil {
		t.Fatalf("Expected DeleteLatency histogram to be non-nil, got nil")
	}

	if metrics1.Keys == nil {
		t.Fatalf("Expected Keys gauge to be non-nil, got nil")
	}

	if metrics1.DiskUsage == nil {
		t.Fatalf("Expected DiskUsage gauge to be non-nil, got nil")
	}

	if metrics1.MemoryUsage == nil {
		t.Fatalf("Expected MemoryUsage gauge to be non-nil, got nil")
	}
}
