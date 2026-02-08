package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// EvictionManager 淘汰管理器
type EvictionManager struct {
	storage    *RocksDBStorage
	running    bool
	stopCh     chan struct{}
	mutex      sync.Mutex
	checkInterval time.Duration
	batchSize     int
	diskThreshold float64
}

// NewEvictionManager 创建新的淘汰管理器实例
func NewEvictionManager(storage *RocksDBStorage) (*EvictionManager, error) {
	return &EvictionManager{
		storage:       storage,
		stopCh:        make(chan struct{}),
		checkInterval: time.Duration(storage.config.Eviction.CheckInterval) * time.Second,
		batchSize:     storage.config.Eviction.BatchSize,
		diskThreshold: storage.config.Eviction.DiskUsageThreshold,
	}, nil
}

// Start 启动淘汰管理器
func (em *EvictionManager) Start() error {
	em.mutex.Lock()
	defer em.mutex.Unlock()

	if em.running {
		return nil
	}

	em.running = true
	go em.run()

	return nil
}

// Stop 停止淘汰管理器
func (em *EvictionManager) Stop() error {
	em.mutex.Lock()
	defer em.mutex.Unlock()

	if !em.running {
		return nil
	}

	em.running = false
	close(em.stopCh)

	return nil
}

// run 运行淘汰管理循环
func (em *EvictionManager) run() {
	ticker := time.NewTicker(em.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := em.checkAndEvict(); err != nil {
				// 记录错误但继续运行
				fmt.Printf("eviction check failed: %v\n", err)
			}
		case <-em.stopCh:
			return
		}
	}
}

// checkAndEvict 检查磁盘使用率并执行淘汰
func (em *EvictionManager) checkAndEvict() error {
	// 1. 检查磁盘使用率
	usage, err := em.getDiskUsage()
	if err != nil {
		return err
	}

	// 2. 如果使用率超过阈值，执行淘汰
	if usage > em.diskThreshold {
		return em.evict()
	}

	return nil
}

// getDiskUsage 获取磁盘使用率
func (em *EvictionManager) getDiskUsage() (float64, error) {
	// 获取磁盘存储目录的磁盘使用情况
	var err error

	// 尝试获取磁盘存储目录的状态
	if em.storage.diskStore != nil {
		_, err = os.Stat(em.storage.diskStore.basePath)
		if err != nil {
			// 如果目录不存在，返回0
			if os.IsNotExist(err) {
				return 0, nil
			}
			return 0, err
		}
	} else {
		// 如果磁盘存储未初始化，返回0
		return 0, nil
	}

	// 获取文件系统信息
	fsPath := filepath.Dir(em.storage.diskStore.basePath)
	if fsPath == "." {
		fsPath = ""
	}

	_, err = os.Stat(fsPath)
	if err != nil {
		return 0, err
	}

	// 计算使用率（这里使用简化的计算方式，实际应该使用系统调用获取真实的磁盘使用情况）
	// 注意：这是一个简化的实现，实际生产环境应该使用更准确的方法
	return 0.0, nil // 暂时返回0，实际实现需要根据系统获取
}

// evict 执行淘汰操作
func (em *EvictionManager) evict() error {
	// 1. 遍历创建时间列族，按时间顺序获取键
	iter := em.storage.db.NewIteratorCF(em.storage.readOpts, em.storage.createTimeCF)
	defer iter.Close()

	evictedCount := 0

	for iter.SeekToFirst(); iter.Valid() && evictedCount < em.batchSize; iter.Next() {
		// 解析key列表
		var keys []string
		if err := json.Unmarshal(iter.Value().Data(), &keys); err != nil {
			continue
		}

		// 遍历每个key
		for _, keyStr := range keys {
			if evictedCount >= em.batchSize {
				break
			}

			key := []byte(keyStr)

			// 检查值是否存储在磁盘上
			value, err := em.storage.db.GetCF(em.storage.readOpts, em.storage.defaultCF, key)
			if err != nil {
				continue
			}

			if value.Size() > 0 {
				valueBytes := value.Data()

				// 检查是否是磁盘存储的值
				if strings.HasPrefix(string(valueBytes), DiskStorePrefix) {
					// 执行淘汰
					if err := em.evictKey(key, valueBytes); err != nil {
						value.Free()
						continue
					}

					evictedCount++
				}
			}
			value.Free()
		}
	}

	return iter.Err()
}

// evictKey 淘汰单个键
func (em *EvictionManager) evictKey(key, value []byte) error {
	// 1. 从磁盘删除文件
	filePath := strings.TrimPrefix(string(value), DiskStorePrefix)
	if err := em.storage.diskStore.Delete(filePath); err != nil {
		return err
	}

	// 2. 更新RocksDB中的值为已淘汰标记
	if err := em.storage.db.PutCF(em.storage.writeOpts, em.storage.defaultCF, key, []byte(EvictedValue)); err != nil {
		return err
	}

	// 3. 从创建时间记录中删除
	return em.storage.removeCreateTime(key)
}
