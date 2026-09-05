package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// DiskStore 磁盘存储实现
type DiskStore struct {
	basePath string
}

// NewDiskStore 创建新的磁盘存储实例
func NewDiskStore(basePath string) (*DiskStore, error) {
	// 确保目录存在
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create disk store directory: %v", err)
	}

	return &DiskStore{
		basePath: basePath,
	}, nil
}

// Store 存储数据到磁盘
func (ds *DiskStore) Store(data []byte) (string, error) {
	// 随机 128-bit 文件名，替代旧的 sha256 内容寻址：
	// sha256 对大 value 要全量哈希（4MB 约 10ms），是写路径第一大 CPU 热点；
	// 随机名让每个 key 独占文件，同时消除"同内容共享文件、删一个 key 弄坏另一个"的隐患。
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", fmt.Errorf("failed to generate file id: %v", err)
	}
	fileName := hex.EncodeToString(id[:])
	filePath := filepath.Join(ds.basePath, fileName)

	// 以 O_SYNC 打开：写入时同步落盘，数据真正刷到磁盘，
	// 不依赖内核 page cache 的延迟回写（此前 os.WriteFile 会把
	// 数据留在脏页，物理落盘被推迟，导致 /sys 磁盘 IO 滞后）。
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|os.O_SYNC, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to open disk file: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(filePath)
		return "", fmt.Errorf("failed to write to disk: %v", err)
	}
	// O_SYNC 已保证单个 write 同步落盘；显式 Sync 作为双保险，
	// 覆盖可能由文件系统合并/延迟提交的极端情况。
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(filePath)
		return "", fmt.Errorf("failed to flush to disk: %v", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("failed to close disk file: %v", err)
	}

	return fileName, nil
}

// Load 从磁盘加载数据
func (ds *DiskStore) Load(fileName string) ([]byte, error) {
	filePath := filepath.Join(ds.basePath, fileName)

	// 读取文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read from disk: %v", err)
	}

	return data, nil
}

// FullPath 返回磁盘文件的绝对路径（sendfile 等零拷贝场景需要按路径直读）
func (ds *DiskStore) FullPath(fileName string) string {
	return filepath.Join(ds.basePath, fileName)
}

// Delete 从磁盘删除数据
func (ds *DiskStore) Delete(fileName string) error {
	filePath := filepath.Join(ds.basePath, fileName)

	// 删除文件
	if err := os.Remove(filePath); err != nil {
		// 忽略文件不存在的错误
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to delete from disk: %v", err)
	}

	return nil
}

// Close 关闭磁盘存储
func (ds *DiskStore) Close() error {
	// 目前不需要特殊处理
	return nil
}
