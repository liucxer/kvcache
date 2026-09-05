package client

import (
	"testing"
	"time"
)

func newTestRegistry(node string) *InstanceRegistry {
	registry := &InstanceRegistry{
		tikvClient:       nil,
		node:             node,
		heartbeatTimeout: 10 * time.Second,
		activeInstances:  make(map[string]*InstanceInfo),
		stopCh:           make(chan struct{}),
	}
	return registry
}

func TestPicker_PrefersLocalInstance(t *testing.T) {
	registry := newTestRegistry("nodeA")

	registry.activeMu.Lock()
	registry.activeInstances["local-1"] = &InstanceInfo{
		Name:      "local-1",
		Node:      "nodeA",
		Addr:      "127.0.0.1:33000",
		Capacity:  1000,
		Available: 500,
		Used:      500,
	}
	registry.activeInstances["remote-1"] = &InstanceInfo{
		Name:      "remote-1",
		Node:      "nodeB",
		Addr:      "192.168.1.2:33000",
		Capacity:  1000,
		Available: 800,
		Used:      200,
	}
	registry.activeMu.Unlock()

	picker := NewInstancePicker(registry, "nodeA", 0.80)

	inst, err := picker.Pick()
	if err != nil {
		t.Fatalf("Failed to pick: %v", err)
	}

	if inst.Node != "nodeA" {
		t.Errorf("Expected local node, got %s", inst.Node)
	}
}

func TestPicker_SkipsOverloadedLocal(t *testing.T) {
	registry := newTestRegistry("nodeA")

	registry.activeMu.Lock()
	registry.activeInstances["local-1"] = &InstanceInfo{
		Name:      "local-1",
		Node:      "nodeA",
		Addr:      "127.0.0.1:33000",
		Capacity:  1000,
		Available: 100,
		Used:      900, // 90% - above threshold
	}
	registry.activeInstances["remote-1"] = &InstanceInfo{
		Name:      "remote-1",
		Node:      "nodeB",
		Addr:      "192.168.1.2:33000",
		Capacity:  1000,
		Available: 700,
		Used:      300, // 30%
	}
	registry.activeMu.Unlock()

	picker := NewInstancePicker(registry, "nodeA", 0.80)

	inst, err := picker.Pick()
	if err != nil {
		t.Fatalf("Failed to pick: %v", err)
	}

	if inst.Name != "remote-1" {
		t.Errorf("Expected remote-1 (less loaded), got %s", inst.Name)
	}
}

func TestPicker_FallbackToLocalWhenAllOverloaded(t *testing.T) {
	registry := newTestRegistry("nodeA")

	registry.activeMu.Lock()
	registry.activeInstances["local-1"] = &InstanceInfo{
		Name:      "local-1",
		Node:      "nodeA",
		Addr:      "127.0.0.1:33000",
		Capacity:  1000,
		Available: 50,
		Used:      950, // 95% - overloaded
	}
	registry.activeInstances["remote-1"] = &InstanceInfo{
		Name:      "remote-1",
		Node:      "nodeB",
		Addr:      "192.168.1.2:33000",
		Capacity:  1000,
		Available: 30,
		Used:      970, // 97% - even more overloaded
	}
	registry.activeMu.Unlock()

	picker := NewInstancePicker(registry, "nodeA", 0.80)

	inst, err := picker.Pick()
	if err != nil {
		t.Fatalf("Failed to pick: %v", err)
	}

	if inst.Node != "nodeA" {
		t.Errorf("Expected fallback to local nodeA, got %s", inst.Node)
	}
}

func TestPicker_NoInstancesReturnsError(t *testing.T) {
	registry := newTestRegistry("nodeA")

	picker := NewInstancePicker(registry, "nodeA", 0.80)

	_, err := picker.Pick()
	if err == nil {
		t.Fatal("Expected error for no instances")
	}
	if err != ErrNoInstances {
		t.Errorf("Expected ErrNoInstances, got %v", err)
	}
}

func TestPicker_OnlyRemoteWhenNoLocal(t *testing.T) {
	registry := newTestRegistry("nodeA")

	registry.activeMu.Lock()
	registry.activeInstances["remote-1"] = &InstanceInfo{
		Name:      "remote-1",
		Node:      "nodeB",
		Addr:      "192.168.1.2:33000",
		Capacity:  1000,
		Available: 700,
		Used:      300,
	}
	registry.activeMu.Unlock()

	picker := NewInstancePicker(registry, "nodeA", 0.80)

	inst, err := picker.Pick()
	if err != nil {
		t.Fatalf("Failed to pick: %v", err)
	}

	if inst.Name != "remote-1" {
		t.Errorf("Expected remote-1, got %s", inst.Name)
	}
}

func TestPicker_DistributesAcrossMultipleLocals(t *testing.T) {
	registry := newTestRegistry("nodeA")

	registry.activeMu.Lock()
	registry.activeInstances["local-1"] = &InstanceInfo{
		Name:      "local-1",
		Node:      "nodeA",
		Addr:      "127.0.0.1:33000",
		Capacity:  1000,
		Available: 500,
		Used:      500,
	}
	registry.activeInstances["local-2"] = &InstanceInfo{
		Name:      "local-2",
		Node:      "nodeA",
		Addr:      "127.0.0.1:33002",
		Capacity:  1000,
		Available: 600,
		Used:      400,
	}
	registry.activeMu.Unlock()

	picker := NewInstancePicker(registry, "nodeA", 0.80)

	counts := make(map[string]int)
	for i := 0; i < 100; i++ {
		inst, err := picker.Pick()
		if err != nil {
			t.Fatalf("Failed to pick: %v", err)
		}
		counts[inst.Name]++
	}

	if counts["local-1"] == 0 {
		t.Error("Expected local-1 to be picked at least once")
	}
	if counts["local-2"] == 0 {
		t.Error("Expected local-2 to be picked at least once")
	}
}

func TestInstanceRegistry_GetActiveInstances(t *testing.T) {
	registry := newTestRegistry("nodeA")

	info1 := &InstanceInfo{Name: "inst1", Node: "nodeA", Addr: "127.0.0.1:33000"}
	info2 := &InstanceInfo{Name: "inst2", Node: "nodeB", Addr: "192.168.1.2:33000"}

	registry.activeMu.Lock()
	registry.activeInstances[info1.Name] = info1
	registry.activeInstances[info2.Name] = info2
	registry.activeMu.Unlock()

	active := registry.GetActiveInstances()
	if len(active) != 2 {
		t.Errorf("Expected 2 active instances, got %d", len(active))
	}

	registry.activeMu.Lock()
	delete(registry.activeInstances, "inst1")
	registry.activeMu.Unlock()

	active = registry.GetActiveInstances()
	if len(active) != 1 {
		t.Errorf("Expected 1 active instance after remove, got %d", len(active))
	}
}

func TestInstanceRegistry_GetLocalAndRemote(t *testing.T) {
	registry := newTestRegistry("nodeA")

	registry.activeMu.Lock()
	registry.activeInstances["a1"] = &InstanceInfo{Name: "a1", Node: "nodeA", Addr: "127.0.0.1:33000"}
	registry.activeInstances["a2"] = &InstanceInfo{Name: "a2", Node: "nodeA", Addr: "127.0.0.1:33002"}
	registry.activeInstances["b1"] = &InstanceInfo{Name: "b1", Node: "nodeB", Addr: "192.168.1.2:33000"}
	registry.activeMu.Unlock()

	local := registry.GetLocalInstances()
	if len(local) != 2 {
		t.Errorf("Expected 2 local instances, got %d", len(local))
	}

	remote := registry.GetRemoteInstances()
	if len(remote) != 1 {
		t.Errorf("Expected 1 remote instance, got %d", len(remote))
	}
}

func TestInstanceInfo_UsagePercent(t *testing.T) {
	info := &InstanceInfo{
		Capacity:  1000,
		Used:      250,
	}

	if info.UsagePercent() != 0.25 {
		t.Errorf("Expected 0.25, got %f", info.UsagePercent())
	}

	info = &InstanceInfo{
		Capacity: 0,
		Used:     0,
	}

	if info.UsagePercent() != 0.0 {
		t.Errorf("Expected 0.0 for zero capacity, got %f", info.UsagePercent())
	}
}
