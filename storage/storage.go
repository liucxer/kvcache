package storage

import (
	"kvcache/config"
)

// Storage 统一存储接口
type Storage interface {
	// 基本操作
	Set(key, value []byte) error
	Get(key []byte) ([]byte, bool, error)
	Delete(key []byte) error
	// RawLookup 定位 key 的存储形态：落盘返回文件绝对路径，内联返回字节
	RawLookup(key []byte) (filePath string, inline []byte, found bool, err error)
	// limit <= 0 表示不限制
	Scan(prefix []byte, limit int) ([][]byte, error)
	ScanWithValues(prefix []byte, limit int) (map[string][]byte, error)

	// 批量操作
	MSet(keyValues map[string][]byte) error
	MGet(keys [][]byte) (map[string][]byte, error)
	MDelete(keys [][]byte) error

	// 配置操作
	GetConfig() (*config.Config, error)
	UpdateConfig(cfg *config.Config) error

	// 管理操作
	Start() error
	Stop() error

	// 获取磁盘容量信息
	GetDiskCapacity() (capacity, available, used int64, err error)

	// 淘汰管理
	StartEvictionManager() error
	StopEvictionManager() error
}

// NewStorage 创建新的存储实例
func NewStorage(cfg *config.Config) (Storage, error) {
	// 1. 创建RocksDB存储实例
	storage, err := NewRocksDBStorage(cfg)
	if err != nil {
		return nil, err
	}

	// 2. 启动存储
	if err := storage.Start(); err != nil {
		return nil, err
	}

	return storage, nil
}
