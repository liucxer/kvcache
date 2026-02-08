package config

import (
	"testing"
)

// TestDefaultConfig 测试默认配置
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// 检查默认配置是否正确
	if cfg.RocksDB.Path != "./data" {
		t.Errorf("Expected RocksDB.Path to be './data', got '%s'", cfg.RocksDB.Path)
	}

	if cfg.Value.DiskPath != "./value_data" {
		t.Errorf("Expected Value.DiskPath to be './value_data', got '%s'", cfg.Value.DiskPath)
	}

	if cfg.Value.DiskThreshold != 1024*1024 {
		t.Errorf("Expected Value.DiskThreshold to be 1048576, got %d", cfg.Value.DiskThreshold)
	}

	if cfg.Eviction.Enabled != true {
		t.Errorf("Expected Eviction.Enabled to be true, got %v", cfg.Eviction.Enabled)
	}

	if cfg.Eviction.DiskUsageThreshold != 0.8 {
		t.Errorf("Expected Eviction.DiskUsageThreshold to be 0.8, got %f", cfg.Eviction.DiskUsageThreshold)
	}

	if cfg.Eviction.CheckInterval != 60 {
		t.Errorf("Expected Eviction.CheckInterval to be 60, got %d", cfg.Eviction.CheckInterval)
	}

	if cfg.Eviction.BatchSize != 100 {
		t.Errorf("Expected Eviction.BatchSize to be 100, got %d", cfg.Eviction.BatchSize)
	}
}

// TestFromJSON 测试从JSON字符串解析配置
func TestFromJSON(t *testing.T) {
	jsonStr := `{
		"rocksdb": {
			"path": "./test/rocksdb"
		},
		"value": {
			"disk_path": "./test/disk_store",
			"disk_threshold": 2048
		},
		"eviction": {
			"enabled": false,
			"disk_usage_threshold": 90.0,
			"check_interval": 30,
			"batch_size": 5
		}
	}`

	cfg, err := FromJSON([]byte(jsonStr))
	if err != nil {
		t.Fatalf("Failed to parse config from JSON: %v", err)
	}

	// 检查解析后的配置是否正确
	if cfg.RocksDB.Path != "./test/rocksdb" {
		t.Errorf("Expected RocksDB.Path to be './test/rocksdb', got '%s'", cfg.RocksDB.Path)
	}

	if cfg.Value.DiskPath != "./test/disk_store" {
		t.Errorf("Expected Value.DiskPath to be './test/disk_store', got '%s'", cfg.Value.DiskPath)
	}

	if cfg.Value.DiskThreshold != 2048 {
		t.Errorf("Expected Value.DiskThreshold to be 2048, got %d", cfg.Value.DiskThreshold)
	}

	if cfg.Eviction.Enabled != false {
		t.Errorf("Expected Eviction.Enabled to be false, got %v", cfg.Eviction.Enabled)
	}

	if cfg.Eviction.DiskUsageThreshold != 90.0 {
		t.Errorf("Expected Eviction.DiskUsageThreshold to be 90.0, got %f", cfg.Eviction.DiskUsageThreshold)
	}

	if cfg.Eviction.CheckInterval != 30 {
		t.Errorf("Expected Eviction.CheckInterval to be 30, got %d", cfg.Eviction.CheckInterval)
	}

	if cfg.Eviction.BatchSize != 5 {
		t.Errorf("Expected Eviction.BatchSize to be 5, got %d", cfg.Eviction.BatchSize)
	}
}

// TestToJSON 测试将配置转换为JSON字符串
func TestToJSON(t *testing.T) {
	cfg := DefaultConfig()

	jsonBytes, err := cfg.ToJSON()
	if err != nil {
		t.Fatalf("Failed to convert config to JSON: %v", err)
	}

	// 检查转换后的JSON字符串是否可以被正确解析
	parsedCfg, err := FromJSON(jsonBytes)
	if err != nil {
		t.Fatalf("Failed to parse config from JSON: %v", err)
	}

	// 检查解析后的配置是否与原始配置相同
	if parsedCfg.RocksDB.Path != cfg.RocksDB.Path {
		t.Errorf("Expected RocksDB.Path to be '%s', got '%s'", cfg.RocksDB.Path, parsedCfg.RocksDB.Path)
	}

	if parsedCfg.Value.DiskPath != cfg.Value.DiskPath {
		t.Errorf("Expected Value.DiskPath to be '%s', got '%s'", cfg.Value.DiskPath, parsedCfg.Value.DiskPath)
	}

	if parsedCfg.Value.DiskThreshold != cfg.Value.DiskThreshold {
		t.Errorf("Expected Value.DiskThreshold to be %d, got %d", cfg.Value.DiskThreshold, parsedCfg.Value.DiskThreshold)
	}

	if parsedCfg.Eviction.Enabled != cfg.Eviction.Enabled {
		t.Errorf("Expected Eviction.Enabled to be %v, got %v", cfg.Eviction.Enabled, parsedCfg.Eviction.Enabled)
	}

	if parsedCfg.Eviction.DiskUsageThreshold != cfg.Eviction.DiskUsageThreshold {
		t.Errorf("Expected Eviction.DiskUsageThreshold to be %f, got %f", cfg.Eviction.DiskUsageThreshold, parsedCfg.Eviction.DiskUsageThreshold)
	}

	if parsedCfg.Eviction.CheckInterval != cfg.Eviction.CheckInterval {
		t.Errorf("Expected Eviction.CheckInterval to be %d, got %d", cfg.Eviction.CheckInterval, parsedCfg.Eviction.CheckInterval)
	}

	if parsedCfg.Eviction.BatchSize != cfg.Eviction.BatchSize {
		t.Errorf("Expected Eviction.BatchSize to be %d, got %d", cfg.Eviction.BatchSize, parsedCfg.Eviction.BatchSize)
	}
}
