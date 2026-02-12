package config

import (
	"encoding/json"
)

// Config 配置结构体
type Config struct {
	RocksDB struct {
		Path    string                 `json:"path"`
		Options map[string]interface{} `json:"options"`
	} `json:"rocksdb"`

	GRPC struct {
		Port int `json:"port"`
	} `json:"grpc"`

	HTTP struct {
		Port int `json:"port"`
	} `json:"http"`

	Batch struct {
		MaxSize int `json:"max_size"`
	} `json:"batch"`

	Value struct {
		DiskThreshold int    `json:"disk_threshold"`
		DiskPath      string `json:"disk_path"`
	} `json:"value"`

	Eviction struct {
		Enabled            bool    `json:"enabled"`
		DiskUsageThreshold float64 `json:"disk_usage_threshold"`
		CheckInterval      int     `json:"check_interval"`
		BatchSize          int     `json:"batch_size"`
	} `json:"eviction"`

	Monitoring struct {
		Enabled     bool   `json:"enabled"`
		MetricsPath string `json:"metrics_path"`
		HealthPath  string `json:"health_path"`
	} `json:"monitoring"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	config := &Config{}

	config.RocksDB.Path = "./data"
	config.RocksDB.Options = make(map[string]interface{})

	config.GRPC.Port = 50051
	config.HTTP.Port = 8080

	config.Batch.MaxSize = 1000

	config.Value.DiskThreshold = 1048576 // 1MB
	config.Value.DiskPath = "./value_data"

	config.Eviction.Enabled = true
	config.Eviction.DiskUsageThreshold = 0.8 // 80%
	config.Eviction.CheckInterval = 60       // 60 seconds
	config.Eviction.BatchSize = 100

	config.Monitoring.Enabled = true
	config.Monitoring.MetricsPath = "/metrics"
	config.Monitoring.HealthPath = "/api/v1/health"

	return config
}

// ToJSON 将配置转换为JSON格式
func (c *Config) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

// FromJSON 从JSON格式加载配置
func FromJSON(data []byte) (*Config, error) {
	config := DefaultConfig()
	err := json.Unmarshal(data, config)
	return config, err
}

// ConfigKey 配置存储在RocksDB中的键
const ConfigKey = "global.config"
