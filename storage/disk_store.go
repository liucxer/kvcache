package storage

import (
	"crypto/sha256"
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
	// 生成唯一文件名（使用SHA256）
	hash := sha256.Sum256(data)
	fileName := hex.EncodeToString(hash[:])
	filePath := filepath.Join(ds.basePath, fileName)

	// 写入文件
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write to disk: %v", err)
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
