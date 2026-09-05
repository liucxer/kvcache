package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tikv/client-go/v2/rawkv"
)

const (
	instanceKeyPrefix = "/kvcache/instances/"
)

type InstanceInfo struct {
	Name      string `json:"name"`
	Node      string `json:"node"`
	Addr      string `json:"addr"`
	RawAddr   string `json:"raw_addr,omitempty"` // 裸 TCP 数据面地址（sendfile 读），空表示不支持
	Capacity  int64  `json:"capacity"`
	Available int64  `json:"available"`
	Used      int64  `json:"used"`
	StartTime int64  `json:"start_time"`
}

func (i *InstanceInfo) UsagePercent() float64 {
	if i.Capacity == 0 {
		return 0.0
	}
	return float64(i.Used) / float64(i.Capacity)
}

type InstanceRegistry struct {
	tikvClient       *rawkv.Client
	heartbeatTimeout time.Duration
	node             string

	activeMu        sync.RWMutex
	activeInstances map[string]*InstanceInfo

	running int32
	stopCh  chan struct{}
}

func NewInstanceRegistry(tikvClient *rawkv.Client, heartbeatTimeout time.Duration, node string) *InstanceRegistry {
	return &InstanceRegistry{
		tikvClient:       tikvClient,
		heartbeatTimeout: heartbeatTimeout,
		node:             node,
		activeInstances:  make(map[string]*InstanceInfo),
		stopCh:           make(chan struct{}),
	}
}

func (ir *InstanceRegistry) Start(ctx context.Context, refreshInterval time.Duration) {
	if !atomic.CompareAndSwapInt32(&ir.running, 0, 1) {
		return
	}

	ir.refreshInstances(ctx)

	go func() {
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ir.refreshInstances(ctx)
			case <-ir.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (ir *InstanceRegistry) Stop() {
	if atomic.CompareAndSwapInt32(&ir.running, 1, 0) {
		close(ir.stopCh)
	}
}

func (ir *InstanceRegistry) refreshInstances(ctx context.Context) {
	prefix := []byte(instanceKeyPrefix)
	keys, values, err := ir.tikvClient.Scan(ctx, prefix, []byte(instanceKeyPrefix+string(rune(0xFF))), 1000)
	if err != nil {
		log.Printf("WARNING: Failed to scan instances from TiKV: %v", err)
		return
	}

	now := time.Now().Unix()
	newActive := make(map[string]*InstanceInfo)

	for i, key := range keys {
		keyStr := string(key)
		if !strings.HasPrefix(keyStr, instanceKeyPrefix) {
			continue
		}

		var info InstanceInfo
		if err := json.Unmarshal(values[i], &info); err != nil {
			log.Printf("WARNING: Failed to parse instance info for %s: %v", keyStr, err)
			continue
		}

		heartbeatAge := now - info.StartTime
		if heartbeatAge > int64(ir.heartbeatTimeout.Seconds()) {
			continue
		}

		newActive[info.Name] = &info
	}

	ir.activeMu.Lock()
	oldActive := ir.activeInstances
	ir.activeInstances = newActive
	ir.activeMu.Unlock()

	for name := range oldActive {
		if _, ok := newActive[name]; !ok {
			log.Printf("INFO: Instance %s marked as offline", name)
		}
	}
}

func (ir *InstanceRegistry) GetActiveInstances() map[string]*InstanceInfo {
	ir.activeMu.RLock()
	defer ir.activeMu.RUnlock()

	result := make(map[string]*InstanceInfo, len(ir.activeInstances))
	for k, v := range ir.activeInstances {
		result[k] = v
	}
	return result
}

func (ir *InstanceRegistry) GetLocalInstances() []*InstanceInfo {
	all := ir.GetActiveInstances()
	var local []*InstanceInfo
	for _, inst := range all {
		if inst.Node == ir.node {
			local = append(local, inst)
		}
	}
	return local
}

func (ir *InstanceRegistry) GetRemoteInstances() []*InstanceInfo {
	all := ir.GetActiveInstances()
	var remote []*InstanceInfo
	for _, inst := range all {
		if inst.Node != ir.node {
			remote = append(remote, inst)
		}
	}
	return remote
}

func (ir *InstanceRegistry) GetOfflineInstances() []string {
	all := ir.GetActiveInstances()

	ir.activeMu.RLock()
	defer ir.activeMu.RUnlock()

	var offline []string
	for name := range ir.activeInstances {
		if _, ok := all[name]; !ok {
			offline = append(offline, name)
		}
	}
	return offline
}

func (ir *InstanceRegistry) Register(ctx context.Context, info *InstanceInfo) error {
	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal instance info: %v", err)
	}

	key := []byte(instanceKeyPrefix + info.Name)
	if err := ir.tikvClient.Put(ctx, key, data); err != nil {
		return fmt.Errorf("failed to register instance: %v", err)
	}

	ir.activeMu.Lock()
	ir.activeInstances[info.Name] = info
	ir.activeMu.Unlock()

	return nil
}

func (ir *InstanceRegistry) Unregister(ctx context.Context, name string) error {
	key := []byte(instanceKeyPrefix + name)
	if err := ir.tikvClient.Delete(ctx, key); err != nil {
		return fmt.Errorf("failed to unregister instance: %v", err)
	}

	ir.activeMu.Lock()
	delete(ir.activeInstances, name)
	ir.activeMu.Unlock()

	return nil
}
