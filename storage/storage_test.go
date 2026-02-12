package storage

import (
	"os"
	"testing"

	"kvcache/config"
)

// TestNewStorage 测试创建存储实例
func TestNewStorage(t *testing.T) {
	// 初始化配置
	cfg := config.DefaultConfig()

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 测试存储是否成功创建
	if store == nil {
		t.Fatalf("Expected store to be non-nil, got nil")
	}
}

// TestStorageSetGet 测试存储的设置和获取功能
func TestStorageSetGet(t *testing.T) {
	// 初始化配置
	cfg := config.DefaultConfig()

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 测试设置值
	testKey := []byte("test-key")
	testValue := []byte("test-value")

	err = store.Set(testKey, testValue)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// 测试获取值
	value, found, err := store.Get(testKey)
	if err != nil {
		t.Fatalf("Failed to get value: %v", err)
	}

	if !found {
		t.Fatalf("Expected key to be found, but it wasn't")
	}

	if string(value) != string(testValue) {
		t.Errorf("Expected value to be '%s', got '%s'", string(testValue), string(value))
	}
}

// TestStorageDelete 测试存储的删除功能
func TestStorageDelete(t *testing.T) {
	// 初始化配置
	cfg := config.DefaultConfig()

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 先设置一个值
	testKey := []byte("test-key")
	testValue := []byte("test-value")

	err = store.Set(testKey, testValue)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// 测试删除值
	err = store.Delete(testKey)
	if err != nil {
		t.Fatalf("Failed to delete value: %v", err)
	}

	// 测试获取已删除的值
	value, found, err := store.Get(testKey)
	if err != nil {
		t.Fatalf("Failed to get value: %v", err)
	}

	if found {
		t.Fatalf("Expected key to be not found, but it was")
	}

	if value != nil {
		t.Errorf("Expected value to be nil, got '%s'", string(value))
	}
}

// TestStorageMSetMGet 测试存储的批量设置和批量获取功能
func TestStorageMSetMGet(t *testing.T) {
	// 初始化配置
	cfg := config.DefaultConfig()

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 测试批量设置值
	testKeyValues := map[string][]byte{
		"batch-key-1": []byte("batch-value-1"),
		"batch-key-2": []byte("batch-value-2"),
	}

	err = store.MSet(testKeyValues)
	if err != nil {
		t.Fatalf("Failed to mset values: %v", err)
	}

	// 测试批量获取值
	testKeys := [][]byte{
		[]byte("batch-key-1"),
		[]byte("batch-key-2"),
	}

	results, err := store.MGet(testKeys)
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

// TestStorageMDelete 测试存储的批量删除功能
func TestStorageMDelete(t *testing.T) {
	// 初始化配置
	cfg := config.DefaultConfig()

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 先批量设置值
	testKeyValues := map[string][]byte{
		"batch-key-1": []byte("batch-value-1"),
		"batch-key-2": []byte("batch-value-2"),
	}

	err = store.MSet(testKeyValues)
	if err != nil {
		t.Fatalf("Failed to mset values: %v", err)
	}

	// 测试批量删除值
	testKeys := [][]byte{
		[]byte("batch-key-1"),
		[]byte("batch-key-2"),
	}

	err = store.MDelete(testKeys)
	if err != nil {
		t.Fatalf("Failed to mdelete values: %v", err)
	}

	// 测试获取已删除的值
	results, err := store.MGet(testKeys)
	if err != nil {
		t.Fatalf("Failed to mget values: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

// TestStorageConfig 测试存储的配置管理功能
func TestStorageConfig(t *testing.T) {
	// 初始化配置
	cfg := config.DefaultConfig()

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 测试获取配置
	storedCfg, err := store.GetConfig()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if storedCfg == nil {
		t.Fatalf("Expected config to be non-nil, got nil")
	}

	// 测试更新配置
	newCfg := config.DefaultConfig()

	err = store.UpdateConfig(newCfg)
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	// 测试获取更新后的配置
	updatedCfg, err := store.GetConfig()
	if err != nil {
		t.Fatalf("Failed to get updated config: %v", err)
	}

	if updatedCfg == nil {
		t.Fatalf("Expected updated config to be non-nil, got nil")
	}
}

// TestStorageScan 测试存储的扫描功能
func TestStorageScan(t *testing.T) {
	// 初始化配置
	cfg := config.DefaultConfig()

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 先设置几个值
	testKeyValues := map[string][]byte{
		"scan-key-1": []byte("scan-value-1"),
		"scan-key-2": []byte("scan-value-2"),
		"other-key":  []byte("other-value"),
	}

	for k, v := range testKeyValues {
		err = store.Set([]byte(k), v)
		if err != nil {
			t.Fatalf("Failed to set value for key '%s': %v", k, err)
		}
	}

	// 测试扫描功能
	keys, err := store.Scan([]byte("scan"))
	if err != nil {
		t.Fatalf("Failed to scan: %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}

	// 检查返回的键是否正确
	foundKeys := make(map[string]bool)
	for _, key := range keys {
		foundKeys[string(key)] = true
	}

	if !foundKeys["scan-key-1"] {
		t.Errorf("Expected 'scan-key-1' to be found")
	}

	if !foundKeys["scan-key-2"] {
		t.Errorf("Expected 'scan-key-2' to be found")
	}
}

// TestStorageDiskStorage 测试磁盘存储功能
func TestStorageDiskStorage(t *testing.T) {
	// 初始化配置，设置较小的磁盘阈值以便测试
	cfg := config.DefaultConfig()
	cfg.Value.DiskThreshold = 10 // 设置为10字节，确保测试值会被存储到磁盘

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 测试大值（超过阈值）
	largeKey := []byte("large-key")
	largeValue := []byte("this is a large value that should be stored on disk") // 长度超过10字节

	err = store.Set(largeKey, largeValue)
	if err != nil {
		t.Fatalf("Failed to set large value: %v", err)
	}

	// 测试获取大值
	retrievedValue, found, err := store.Get(largeKey)
	if err != nil {
		t.Fatalf("Failed to get large value: %v", err)
	}

	if !found {
		t.Fatalf("Expected key to be found, but it wasn't")
	}

	if string(retrievedValue) != string(largeValue) {
		t.Errorf("Expected value to be '%s', got '%s'", string(largeValue), string(retrievedValue))
	}
}

// TestStorageEvictionManager 测试淘汰管理器功能
func TestStorageEvictionManager(t *testing.T) {
	// 初始化配置，启用淘汰机制
	cfg := config.DefaultConfig()
	cfg.Eviction.Enabled = true

	// 删除现有的数据目录，确保测试环境干净
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	store, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 测试启动淘汰管理器
	err = store.StartEvictionManager()
	if err != nil {
		t.Fatalf("Failed to start eviction manager: %v", err)
	}

	// 测试停止淘汰管理器
	err = store.StopEvictionManager()
	if err != nil {
		t.Fatalf("Failed to stop eviction manager: %v", err)
	}

	// 测试再次启动淘汰管理器
	err = store.StartEvictionManager()
	if err != nil {
		t.Fatalf("Failed to start eviction manager again: %v", err)
	}
}

// TestDiskStore 测试磁盘存储功能
func TestDiskStore(t *testing.T) {
	// 创建临时目录用于测试
	tempDir, err := os.MkdirTemp("", "diskstore-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建磁盘存储实例
	diskStore, err := NewDiskStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create disk store: %v", err)
	}
	defer diskStore.Close()

	// 测试存储数据
	testData := []byte("test data for disk store")
	fileName, err := diskStore.Store(testData)
	if err != nil {
		t.Fatalf("Failed to store data: %v", err)
	}

	if fileName == "" {
		t.Fatalf("Expected non-empty file name, got empty")
	}

	// 测试加载数据
	loadedData, err := diskStore.Load(fileName)
	if err != nil {
		t.Fatalf("Failed to load data: %v", err)
	}

	if string(loadedData) != string(testData) {
		t.Errorf("Expected data to be '%s', got '%s'", string(testData), string(loadedData))
	}

	// 测试删除数据
	err = diskStore.Delete(fileName)
	if err != nil {
		t.Fatalf("Failed to delete data: %v", err)
	}

	// 测试删除不存在的文件（应该成功）
	err = diskStore.Delete("non-existent-file")
	if err != nil {
		t.Fatalf("Expected success for deleting non-existent file, but got error: %v", err)
	}

	// 测试加载已删除的数据（应该失败）
	_, err = diskStore.Load(fileName)
	if err == nil {
		t.Fatalf("Expected error for loading deleted data, but got nil")
	}
}
