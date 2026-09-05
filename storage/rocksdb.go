package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"kvcache/config"
	"sync"
	"syscall"
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

// 预计算的字节形式标记，避免热路径上 []byte→string 转换带来的堆分配
var (
	diskStorePrefixBytes = []byte(DiskStorePrefix)
	evictedValueBytes    = []byte(EvictedValue)
	configKeyBytes       = []byte(config.ConfigKey)
)

// RocksDBStorage RocksDB存储实现
type RocksDBStorage struct {
	db           *gorocksdb.DB
	opts         *gorocksdb.Options
	cfOpts       *gorocksdb.Options
	readOpts     *gorocksdb.ReadOptions
	scanOpts     *gorocksdb.ReadOptions // 扫描专用：不填充 block cache，避免冲掉点查热数据
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
	if s.scanOpts != nil {
		s.scanOpts.Destroy()
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

	// 性能优化
	s.opts.SetAllowConcurrentMemtableWrites(true)
	s.opts.SetEnablePipelinedWrite(true)
	s.opts.SetAllowMmapWrites(true)
	s.opts.SetAllowMmapReads(true)

	// Write buffer: 256MB memtable, 4 memtables
	s.opts.SetWriteBufferSize(256 * 1024 * 1024)
	s.opts.SetMaxWriteBufferNumber(4)
	s.opts.SetMinWriteBufferNumberToMerge(2)

	// Background jobs: 8 workers for flush + compaction
	s.opts.SetMaxBackgroundJobs(8)
	s.opts.SetMaxBackgroundCompactions(6)

	// Block cache: 走配置 block_cache_size（MB），未配置时保持旧的 1GB 行为
	blockCacheSizeMB := s.config.RocksDB.BlockCacheSize
	if blockCacheSizeMB <= 0 {
		blockCacheSizeMB = 1024
	}
	blockCache := gorocksdb.NewLRUCache(uint64(blockCacheSizeMB) * 1024 * 1024)
	bbto := gorocksdb.NewDefaultBlockBasedTableOptions()
	bbto.SetBlockCache(blockCache)
	bbto.SetBlockSize(64 * 1024) // 64KB block size
	// Bloom filter（10 bits/key，~1% 误判）：缓存场景 Get miss 是常态，
	// 没有 bloom filter 时 miss 会穿透多层 SST 做磁盘查找
	bbto.SetFilterPolicy(gorocksdb.NewBloomFilter(10))
	s.opts.SetBlockBasedTableFactory(bbto)

	// Compaction: level compaction with 64MB target file size
	s.opts.SetTargetFileSizeBase(64 * 1024 * 1024)
	s.opts.SetMaxBytesForLevelBase(512 * 1024 * 1024)

	log.Printf("RocksDB tuned: write_buffer=256MB x 4, block_cache=%dMB, bloom_filter=10bits, max_bg_jobs=8", blockCacheSizeMB)

	// 初始化选项
	s.cfOpts = gorocksdb.NewDefaultOptions()
	s.cfOpts.SetAllowConcurrentMemtableWrites(true)
	s.cfOpts.SetWriteBufferSize(256 * 1024 * 1024)
	s.cfOpts.SetMaxWriteBufferNumber(4)
	cfBbto := gorocksdb.NewDefaultBlockBasedTableOptions()
	cfBbto.SetBlockCache(blockCache)
	cfBbto.SetBlockSize(64 * 1024)
	// 注意 filter policy 是 move 语义，不能共享，需单独实例
	cfBbto.SetFilterPolicy(gorocksdb.NewBloomFilter(10))
	s.cfOpts.SetBlockBasedTableFactory(cfBbto)

	s.readOpts = gorocksdb.NewDefaultReadOptions()
	s.readOpts.SetFillCache(true)
	s.scanOpts = gorocksdb.NewDefaultReadOptions()
	s.scanOpts.SetFillCache(false) // 扫描不填充 block cache，避免冲掉点查热数据
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

	// 2. 检查值类型（bytes 比较，避免热路径上 []byte→string 的堆分配）
	if bytes.Equal(valueBytes, evictedValueBytes) {
		return nil, true, fmt.Errorf("value has been evicted")
	}

	if bytes.HasPrefix(valueBytes, diskStorePrefixBytes) {
		// 从磁盘获取
		filePath := string(valueBytes[len(diskStorePrefixBytes):])
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

// RawLookup 定位 key 的存储形态（不加载大 value 内容）：
// 落盘大 value 返回文件绝对路径；内联小 value 返回拷贝后的字节；
// 未找到/已淘汰返回 found=false。供裸 TCP 数据面 sendfile 使用。
func (s *RocksDBStorage) RawLookup(key []byte) (filePath string, inline []byte, found bool, err error) {
	value, err := s.db.GetCF(s.readOpts, s.defaultCF, key)
	if err != nil {
		return "", nil, false, err
	}
	defer value.Free()

	if value.Size() == 0 {
		return "", nil, false, nil
	}

	valueBytes := value.Data()
	if bytes.Equal(valueBytes, evictedValueBytes) {
		return "", nil, false, nil
	}

	if bytes.HasPrefix(valueBytes, diskStorePrefixBytes) {
		fileName := string(valueBytes[len(diskStorePrefixBytes):])
		return s.diskStore.FullPath(fileName), nil, true, nil
	}

	// 内联小 value：拷贝，因为 value.Free() 会释放内部缓冲区
	inline = make([]byte, len(valueBytes))
	copy(inline, valueBytes)
	return "", inline, true, nil
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
		if bytes.HasPrefix(valueBytes, diskStorePrefixBytes) {
			// 删除磁盘文件
			filePath := string(valueBytes[len(diskStorePrefixBytes):])
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

// Scan 扫描键前缀（只返回 key，不加载 value）。
// 从 prefix 处 Seek 定位、不匹配即 break，避免全表扫描。
// limit <= 0 表示不限制。使用 scanOpts（不填充 block cache）。
func (s *RocksDBStorage) Scan(prefix []byte, limit int) ([][]byte, error) {
	iter := s.db.NewIteratorCF(s.scanOpts, s.defaultCF)
	defer iter.Close()

	var keys [][]byte
	count := 0

	for iter.Seek(prefix); iter.Valid(); iter.Next() {
		key := iter.Key().Data()
		if !bytes.HasPrefix(key, prefix) {
			break
		}
		// 跳过配置键
		if bytes.Equal(key, configKeyBytes) {
			continue
		}
		// 复制键，因为 iter.Key().Data() 会在 iter.Next() 后失效
		keyCopy := make([]byte, len(key))
		copy(keyCopy, key)
		keys = append(keys, keyCopy)
		count++
		if limit > 0 && count >= limit {
			break
		}
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return keys, nil
}

// ScanWithValues 扫描键前缀并返回值。
// 直接从迭代器取 value，消除旧实现"每个 key 再 Get 一次"的 N+1 问题；
// 只有磁盘大值引用才去读文件。limit <= 0 表示不限制。
func (s *RocksDBStorage) ScanWithValues(prefix []byte, limit int) (map[string][]byte, error) {
	iter := s.db.NewIteratorCF(s.scanOpts, s.defaultCF)
	defer iter.Close()

	keyValues := make(map[string][]byte)
	count := 0

	for iter.Seek(prefix); iter.Valid(); iter.Next() {
		key := iter.Key().Data()
		if !bytes.HasPrefix(key, prefix) {
			break
		}
		// 跳过配置键
		if bytes.Equal(key, configKeyBytes) {
			continue
		}

		value := iter.Value().Data()
		switch {
		case bytes.Equal(value, evictedValueBytes):
			continue // 已淘汰的 key 不返回
		case bytes.HasPrefix(value, diskStorePrefixBytes):
			// 磁盘大值：读文件
			filePath := string(value[len(diskStorePrefixBytes):])
			diskValue, err := s.diskStore.Load(filePath)
			if err != nil {
				continue // 跳过读取失败的键
			}
			keyValues[string(key)] = diskValue
		default:
			// 复制值，因为 iter.Value().Data() 会在 iter.Next() 后失效
			valueCopy := make([]byte, len(value))
			copy(valueCopy, value)
			keyValues[string(key)] = valueCopy
		}
		count++
		if limit > 0 && count >= limit {
			break
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

// MGet 批量获取值。
// 一次 MultiGetCF 拿到所有小值/磁盘引用（替代旧的逐 key Get），
// 磁盘大值再并发读文件（最多 8 路，避免并发突刺打满磁盘）。
func (s *RocksDBStorage) MGet(keys [][]byte) (map[string][]byte, error) {
	keyValues := make(map[string][]byte)
	if len(keys) == 0 {
		return keyValues, nil
	}

	slices, err := s.db.MultiGetCF(s.readOpts, s.defaultCF, keys...)
	if err != nil {
		return nil, err
	}
	defer func() {
		for _, sl := range slices {
			sl.Free()
		}
	}()

	type diskRef struct {
		key      string
		filePath string
	}
	var diskRefs []diskRef

	for i, sl := range slices {
		if sl.Size() == 0 {
			continue // key 不存在
		}
		valueBytes := sl.Data()
		if bytes.Equal(valueBytes, evictedValueBytes) {
			continue // 已淘汰
		}
		if bytes.HasPrefix(valueBytes, diskStorePrefixBytes) {
			diskRefs = append(diskRefs, diskRef{
				key:      string(keys[i]),
				filePath: string(valueBytes[len(diskStorePrefixBytes):]),
			})
			continue
		}
		// 复制值，Slice.Free 后内部缓冲区失效
		valueCopy := make([]byte, len(valueBytes))
		copy(valueCopy, valueBytes)
		keyValues[string(keys[i])] = valueCopy
	}

	// 并发加载磁盘大值
	if len(diskRefs) > 0 {
		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)
		for _, ref := range diskRefs {
			wg.Add(1)
			go func(ref diskRef) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				data, err := s.diskStore.Load(ref.filePath)
				if err != nil {
					return // 跳过读取失败的键
				}
				mu.Lock()
				keyValues[ref.key] = data
				mu.Unlock()
			}(ref)
		}
		wg.Wait()
	}

	return keyValues, nil
}

// MDelete 批量删除键值对。
// 先用一次 MultiGetCF 找出磁盘大值引用（替代旧的逐 key Get），
// 磁盘文件并发删除，RocksDB 侧走单个 WriteBatch 原子提交。
func (s *RocksDBStorage) MDelete(keys [][]byte) error {
	if len(keys) == 0 {
		return nil
	}

	slices, err := s.db.MultiGetCF(s.readOpts, s.defaultCF, keys...)
	if err != nil {
		return err
	}

	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()

	var wg sync.WaitGroup
	for i, sl := range slices {
		if sl.Size() > 0 {
			valueBytes := sl.Data()
			if bytes.HasPrefix(valueBytes, diskStorePrefixBytes) {
				// 并发删除磁盘文件
				filePath := string(valueBytes[len(diskStorePrefixBytes):])
				wg.Add(1)
				go func(fp string) {
					defer wg.Done()
					_ = s.diskStore.Delete(fp)
				}(filePath)
			}
		}
		sl.Free()

		// 从RocksDB删除
		wb.DeleteCF(s.defaultCF, keys[i])

		// 从创建时间记录中删除
		if err := s.removeCreateTime(keys[i]); err != nil {
			continue
		}
	}
	wg.Wait()

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

// GetDiskCapacity 获取磁盘容量信息
func (s *RocksDBStorage) GetDiskCapacity() (capacity, available, used int64, err error) {
	dataPath := s.config.RocksDB.Path

	stat := &syscall.Statfs_t{}
	if err = syscall.Statfs(dataPath, stat); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get disk capacity: %v", err)
	}

	capacity = int64(stat.Blocks) * int64(stat.Bsize)
	available = int64(stat.Bavail) * int64(stat.Bsize)
	used = capacity - available

	return capacity, available, used, nil
}
