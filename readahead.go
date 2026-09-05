package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// 块设备默认 readahead 窗口 128KB，会把 4MB 整文件顺序读（裸数据面 sendfile）
// 拆成 32 个碎 IO，实测单盘带宽从 3.3GB/s 掉到 2.2GB/s（-33%）。
// 启动时把 value-dir 所在块设备的 read_ahead_kb 调大，让整文件读聚合成大 IO。
// 属于性能调优而非功能依赖：任何一步失败都只告警、不阻断启动。

// setReadaheadForPath 将 path 所在块设备的 read_ahead_kb 设为 kb
func setReadaheadForPath(path string, kb int) {
	devName, err := blockDeviceOf(path)
	if err != nil {
		log.Printf("WARNING: detect block device for %s failed, skip readahead tuning: %v", path, err)
		return
	}

	queueFile := filepath.Join("/sys/class/block", devName, "queue", "read_ahead_kb")
	data, err := os.ReadFile(queueFile)
	if err != nil {
		log.Printf("WARNING: read %s failed, skip readahead tuning: %v", queueFile, err)
		return
	}
	cur, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	if cur == kb {
		log.Printf("read_ahead_kb for %s (%s) already %d", devName, path, kb)
		return
	}

	if err := os.WriteFile(queueFile, []byte(strconv.Itoa(kb)), 0644); err != nil {
		log.Printf("WARNING: set %s=%d failed (need root): %v", queueFile, kb, err)
		return
	}
	log.Printf("Tuned read_ahead_kb for %s (%s): %d -> %d", devName, path, cur, kb)
}

// blockDeviceOf 返回 path 所在块设备的磁盘名（分区归并到父盘，如 nvme5n1p1 → nvme5n1）
func blockDeviceOf(path string) (string, error) {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return "", err
	}

	// Linux dev_t 解码（同 gnu_dev_major/minor 宏）
	dev := uint64(st.Dev)
	major := (uint32(dev>>8) & 0xfff) | (uint32(dev>>32) &^ 0xfff)
	minor := (uint32(dev) & 0xff) | (uint32(dev>>12) &^ 0xff)

	link, err := os.Readlink(fmt.Sprintf("/sys/dev/block/%d:%d", major, minor))
	if err != nil {
		return "", fmt.Errorf("resolve /sys/dev/block/%d:%d: %v", major, minor, err)
	}
	abs := filepath.Clean(filepath.Join("/sys/dev/block", link))
	name := filepath.Base(abs)

	// 分区没有独立的 queue 目录，归并到父磁盘
	if _, err := os.Stat(filepath.Join("/sys/class/block", name, "partition")); err == nil {
		name = filepath.Base(filepath.Dir(abs))
	}
	return name, nil
}
