package storage

import (
	"kvcache/config"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	gorocksdb "github.com/linxGnu/grocksdb"
)

const (
	// DiskStorePrefix 磁盘存储路径前缀
	DiskStorePrefix = "__rocksdb_disk_store__://"
	// EvictedValue 已淘汰值标记
	EvictedValue = "__evicted__"
	// CreateTimeCF 创建时间列族
	CreateTimeCF = "create_time"
	// MetadataCF 元数据列族
	MetadataCF = "metadata"
)

// RocksDBStorage RocksDB存储实现
type RocksDBStorage struct {
	db           *gorocksdb.DB
	opts         *gorocksdb.Options
	cfOpts       *gorocksdb.Options
	readOpts     *gorocksdb.ReadOptions
	writeOpts    *gorocksdb.WriteOptions
	defaultCF    *gorocksdb.ColumnFamilyHandle
	createTimeCF *gorocksdb.ColumnFamilyHandle
	metadataCF   *gorocksdb.ColumnFamilyHandle
	config       *config.Config
	diskStore    *DiskStore
	eviction     *EvictionManager
}

// NewRocksDBStorage 创建新的RocksDB存储实例
func NewRocksDBStorage(cfg *config.Config) (*RocksDBStorage, error) {
	storage := &RocksDBStorage{
		config: cfg,
	}

	return storage, nil
}

// Start 启动存储
func (s *RocksDBStorage) Start() error {
	// 1. 初始化RocksDB
	if err := s.initRocksDB(); err != nil {
		return err
	}

	// 2. 初始化磁盘存储
	diskStore, err := NewDiskStore(s.config.Value.DiskPath)
	if err != nil {
		return err
	}
	s.diskStore = diskStore

	// 3. 加载配置
	if err := s.loadConfig(); err != nil {
		return err
	}

	// 4. 存储配置到RocksDB
	if err := s.storeConfig(); err != nil {
		return err
	}

	// 5. 检查是否启用淘汰机制
	if s.config.Eviction.Enabled {
		if err := s.StartEvictionManager(); err != nil {
			return err
		}
	}

	return nil
}

// Stop 停止存储
func (s *RocksDBStorage) Stop() error {
	// 停止淘汰管理器
	s.StopEvictionManager()

	// 关闭磁盘存储
	if s.diskStore != nil {
		s.diskStore.Close()
	}

	// 关闭RocksDB
	if s.defaultCF != nil {
		s.defaultCF.Destroy()
	}
	if s.createTimeCF != nil {
		s.createTimeCF.Destroy()
	}
	if s.metadataCF != nil {
		s.metadataCF.Destroy()
	}
	if s.db != nil {
		s.db.Close()
	}
	if s.opts != nil {
		s.opts.Destroy()
	}
	if s.cfOpts != nil {
		s.cfOpts.Destroy()
	}
	if s.readOpts != nil {
		s.readOpts.Destroy()
	}
	if s.writeOpts != nil {
		s.writeOpts.Destroy()
	}

	return nil
}

// initRocksDB 初始化RocksDB
func (s *RocksDBStorage) initRocksDB() error {
	// 1. 创建选项
	s.opts = gorocksdb.NewDefaultOptions()
	s.opts.SetCreateIfMissing(true)

	s.cfOpts = gorocksdb.NewDefaultOptions()
	s.readOpts = gorocksdb.NewDefaultReadOptions()
	s.writeOpts = gorocksdb.NewDefaultWriteOptions()

	// 2. 准备要使用的列族
	cfNames := []string{"default"}
	cfOpts := make([]*gorocksdb.Options, len(cfNames))
	for i := range cfOpts {
		cfOpts[i] = s.cfOpts
	}

	// 3. 打开数据库，只使用default列族
	db, cfHandles, err := gorocksdb.OpenDbColumnFamilies(s.opts, s.config.RocksDB.Path, cfNames, cfOpts)
	if err != nil {
		return fmt.Errorf("failed to open rocksdb: %v", err)
	}

	// 4. 赋值
	s.db = db
	s.defaultCF = cfHandles[0]
	// 暂时不使用其他列族，后续需要时再创建

	return nil
}

// Set 设置键值对
func (s *RocksDBStorage) Set(key, value []byte) error {
	// 1. 检查是否需要存储到磁盘
	if len(value) > s.config.Value.DiskThreshold {
		// 存储到磁盘
		filePath, err := s.diskStore.Store(value)
		if err != nil {
			return err
		}

		// 在RocksDB中存储路径
		diskValue := []byte(DiskStorePrefix + filePath)
		if err := s.db.PutCF(s.writeOpts, s.defaultCF, key, diskValue); err != nil {
			return err
		}
	} else {
		// 直接存储到RocksDB
		if err := s.db.PutCF(s.writeOpts, s.defaultCF, key, value); err != nil {
			return err
		}
	}

	// 2. 记录创建时间
	if err := s.recordCreateTime(key); err != nil {
		return err
	}

	return nil
}

// Get 获取值
func (s *RocksDBStorage) Get(key []byte) ([]byte, bool, error) {
	// 1. 从RocksDB获取
	value, err := s.db.GetCF(s.readOpts, s.defaultCF, key)
	if err != nil {
		return nil, false, err
	}
	defer value.Free()

	if value.Size() == 0 {
		return nil, false, nil
	}

	valueBytes := value.Data()

	// 2. 检查值类型
	if string(valueBytes) == EvictedValue {
		return nil, true, fmt.Errorf("value has been evicted")
	}

	if strings.HasPrefix(string(valueBytes), DiskStorePrefix) {
		// 从磁盘获取
		filePath := strings.TrimPrefix(string(valueBytes), DiskStorePrefix)
		diskValue, err := s.diskStore.Load(filePath)
		if err != nil {
			return nil, true, err
		}
		return diskValue, true, nil
	}

	// 复制valueBytes的内容，因为value.Free()会释放内部缓冲区
	copyValue := make([]byte, len(valueBytes))
	copy(copyValue, valueBytes)

	// 返回复制后的值
	return copyValue, true, nil
}

// Delete 删除键值对
func (s *RocksDBStorage) Delete(key []byte) error {
	// 1. 先获取值，检查是否存储在磁盘
	value, err := s.db.GetCF(s.readOpts, s.defaultCF, key)
	if err != nil {
		return err
	}
	defer value.Free()

	if value.Size() > 0 {
		valueBytes := value.Data()
		if strings.HasPrefix(string(valueBytes), DiskStorePrefix) {
			// 删除磁盘文件
			filePath := strings.TrimPrefix(string(valueBytes), DiskStorePrefix)
			s.diskStore.Delete(filePath)
		}
	}

	// 2. 从RocksDB删除
	if err := s.db.DeleteCF(s.writeOpts, s.defaultCF, key); err != nil {
		return err
	}

	// 3. 从创建时间记录中删除
	if err := s.removeCreateTime(key); err != nil {
		return err
	}

	return nil
}

// Scan 扫描键前缀
func (s *RocksDBStorage) Scan(prefix []byte) ([][]byte, error) {
	iter := s.db.NewIteratorCF(s.readOpts, s.defaultCF)
	defer iter.Close()

	var keys [][]byte
	prefixStr := string(prefix)

	for iter.Seek(prefix); iter.Valid(); iter.Next() {
		key := iter.Key().Data()
		keyStr := string(key)

		if !strings.HasPrefix(keyStr, prefixStr) {
			break
		}

		// 跳过配置键
		if keyStr == config.ConfigKey {
			continue
		}

		keys = append(keys, key)
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return keys, nil
}

// ScanWithValues 扫描键前缀并返回值
func (s *RocksDBStorage) ScanWithValues(prefix []byte) (map[string][]byte, error) {
	iter := s.db.NewIteratorCF(s.readOpts, s.defaultCF)
	defer iter.Close()

	keyValues := make(map[string][]byte)
	prefixStr := string(prefix)

	for iter.Seek(prefix); iter.Valid(); iter.Next() {
		key := iter.Key().Data()
		keyStr := string(key)

		if !strings.HasPrefix(keyStr, prefixStr) {
			break
		}

		// 跳过配置键
		if keyStr == config.ConfigKey {
			continue
		}

		// 获取值
		value, found, err := s.Get(key)
		if err != nil {
			continue // 跳过错误的键
		}
		if found {
			keyValues[keyStr] = value
		}
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return keyValues, nil
}

// MSet 批量设置键值对
func (s *RocksDBStorage) MSet(keyValues map[string][]byte) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()

	for k, v := range keyValues {
		key := []byte(k)

		// 检查是否需要存储到磁盘
		if len(v) > s.config.Value.DiskThreshold {
			// 存储到磁盘
			filePath, err := s.diskStore.Store(v)
			if err != nil {
				return err
			}

			// 在RocksDB中存储路径
			diskValue := []byte(DiskStorePrefix + filePath)
			wb.PutCF(s.defaultCF, key, diskValue)
		} else {
			// 直接存储到RocksDB
			wb.PutCF(s.defaultCF, key, v)
		}

		// 记录创建时间
		if err := s.recordCreateTime(key); err != nil {
			return err
		}
	}

	return s.db.Write(s.writeOpts, wb)
}

// MGet 批量获取值
func (s *RocksDBStorage) MGet(keys [][]byte) (map[string][]byte, error) {
	keyValues := make(map[string][]byte)

	for _, key := range keys {
		value, found, err := s.Get(key)
		if err == nil && found {
			keyValues[string(key)] = value
		}
	}

	return keyValues, nil
}

// MDelete 批量删除键值对
func (s *RocksDBStorage) MDelete(keys [][]byte) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()

	for _, key := range keys {
		// 先获取值，检查是否存储在磁盘
		value, err := s.db.GetCF(s.readOpts, s.defaultCF, key)
		if err != nil {
			continue
		}

		if value.Size() > 0 {
			valueBytes := value.Data()
			if strings.HasPrefix(string(valueBytes), DiskStorePrefix) {
				// 删除磁盘文件
				filePath := strings.TrimPrefix(string(valueBytes), DiskStorePrefix)
				s.diskStore.Delete(filePath)
			}
		}
		value.Free()

		// 从RocksDB删除
		wb.DeleteCF(s.defaultCF, key)

		// 从创建时间记录中删除
		if err := s.removeCreateTime(key); err != nil {
			continue
		}
	}

	return s.db.Write(s.writeOpts, wb)
}

// GetConfig 获取配置
func (s *RocksDBStorage) GetConfig() (*config.Config, error) {
	// 如果metadataCF为nil，返回默认配置
	if s.metadataCF == nil {
		return config.DefaultConfig(), nil
	}

	value, err := s.db.GetCF(s.readOpts, s.metadataCF, []byte(config.ConfigKey))
	if err != nil {
		return nil, err
	}
	defer value.Free()

	if value.Size() == 0 {
		// 返回默认配置
		return config.DefaultConfig(), nil
	}

	// 解析配置
	cfg, err := config.FromJSON(value.Data())
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// UpdateConfig 更新配置
func (s *RocksDBStorage) UpdateConfig(cfg *config.Config) error {
	// 序列化配置
	configBytes, err := cfg.ToJSON()
	if err != nil {
		return err
	}

	// 如果metadataCF不为nil，存储到RocksDB
	if s.metadataCF != nil {
		if err := s.db.PutCF(s.writeOpts, s.metadataCF, []byte(config.ConfigKey), configBytes); err != nil {
			return err
		}
	}

	// 更新内存配置
	s.config = cfg

	// 重启淘汰管理器
	if cfg.Eviction.Enabled {
		s.StopEvictionManager()
		if err := s.StartEvictionManager(); err != nil {
			return err
		}
	} else {
		s.StopEvictionManager()
	}

	return nil
}

// recordCreateTime 记录创建时间
func (s *RocksDBStorage) recordCreateTime(key []byte) error {
	// 如果createTimeCF为nil，跳过记录创建时间
	if s.createTimeCF == nil {
		return nil
	}

	// 获取当前时间戳（秒）
	timestamp := time.Now().Unix()
	timestampKey := []byte(fmt.Sprintf("%d", timestamp))

	// 读取当前时间戳的key列表
	value, err := s.db.GetCF(s.readOpts, s.createTimeCF, timestampKey)
	if err != nil {
		return err
	}

	var keys []string
	if value.Size() > 0 {
		if err := json.Unmarshal(value.Data(), &keys); err != nil {
			value.Free()
			return err
		}
	}
	value.Free()

	// 添加新key
	keys = append(keys, string(key))

	// 写回
	keysBytes, err := json.Marshal(keys)
	if err != nil {
		return err
	}

	return s.db.PutCF(s.writeOpts, s.createTimeCF, timestampKey, keysBytes)
}

// removeCreateTime 从创建时间记录中删除
func (s *RocksDBStorage) removeCreateTime(key []byte) error {
	// 如果createTimeCF为nil，跳过删除创建时间记录
	if s.createTimeCF == nil {
		return nil
	}

	// 遍历所有时间戳
	iter := s.db.NewIteratorCF(s.readOpts, s.createTimeCF)
	defer iter.Close()

	keyStr := string(key)

	for iter.SeekToFirst(); iter.Valid(); iter.Next() {
		timestampKey := iter.Key().Data()
		value := iter.Value().Data()

		var keys []string
		if err := json.Unmarshal(value, &keys); err != nil {
			continue
		}

		// 查找并删除key
		newKeys := make([]string, 0)
		for _, k := range keys {
			if k != keyStr {
				newKeys = append(newKeys, k)
			}
		}

		// 如果key被删除了，更新记录
		if len(newKeys) != len(keys) {
			if len(newKeys) == 0 {
				// 如果没有key了，删除整个记录
				if err := s.db.DeleteCF(s.writeOpts, s.createTimeCF, timestampKey); err != nil {
					return err
				}
			} else {
				// 更新记录
				newKeysBytes, err := json.Marshal(newKeys)
				if err != nil {
					return err
				}

				if err := s.db.PutCF(s.writeOpts, s.createTimeCF, timestampKey, newKeysBytes); err != nil {
					return err
				}
			}
			break
		}
	}

	return nil
}

// StartEvictionManager 启动淘汰管理器
func (s *RocksDBStorage) StartEvictionManager() error {
	eviction, err := NewEvictionManager(s)
	if err != nil {
		return err
	}

	s.eviction = eviction
	return s.eviction.Start()
}

// StopEvictionManager 停止淘汰管理器
func (s *RocksDBStorage) StopEvictionManager() error {
	if s.eviction != nil {
		return s.eviction.Stop()
	}
	return nil
}

// loadConfig 加载配置
func (s *RocksDBStorage) loadConfig() error {
	cfg, err := s.GetConfig()
	if err != nil {
		return err
	}

	s.config = cfg
	return nil
}

// storeConfig 存储配置
func (s *RocksDBStorage) storeConfig() error {
	return s.UpdateConfig(s.config)
}
