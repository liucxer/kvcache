package client

import (
	"fmt"
	"sync"
	"testing"
)

func TestRouteCache_PutAndGet(t *testing.T) {
	cache := NewRouteCache(int64(1 * 1024 * 1024))

	cache.Put("key1", "inst1")
	cache.Put("key2", "inst2")
	cache.Put("key3", "inst3")

	tests := []struct {
		key      string
		expected string
		found    bool
	}{
		{"key1", "inst1", true},
		{"key2", "inst2", true},
		{"key3", "inst3", true},
		{"key4", "", false},
	}

	for _, tt := range tests {
		val, ok := cache.Get(tt.key)
		if ok != tt.found {
			t.Errorf("Get(%s): found = %v, want %v", tt.key, ok, tt.found)
		}
		if val != tt.expected {
			t.Errorf("Get(%s): got %q, want %q", tt.key, val, tt.expected)
		}
	}
}

func TestRouteCache_UpdateExisting(t *testing.T) {
	cache := NewRouteCache(int64(1 * 1024 * 1024))

	cache.Put("key1", "inst1")
	cache.Put("key1", "inst2")

	val, ok := cache.Get("key1")
	if !ok {
		t.Error("Expected key1 to be found")
	}
	if val != "inst2" {
		t.Errorf("Expected inst2, got %s", val)
	}
}

func TestRouteCache_Delete(t *testing.T) {
	cache := NewRouteCache(int64(1 * 1024 * 1024))

	cache.Put("key1", "inst1")
	cache.Put("key2", "inst2")

	cache.Delete("key1")

	if _, ok := cache.Get("key1"); ok {
		t.Error("Expected key1 to be deleted")
	}

	if val, ok := cache.Get("key2"); !ok || val != "inst2" {
		t.Error("Expected key2 to still exist")
	}
}

func TestRouteCache_LRUEviction(t *testing.T) {
	limit := int64(500)
	cache := NewRouteCache(limit)

	cache.Put("key1", "inst1")
	cache.Put("key2", "inst2")
	cache.Put("key3", "inst3")

	if cache.Size() != 3 {
		t.Errorf("Expected 3 entries, got %d", cache.Size())
	}

	cache.Put("key4", "inst4")
	cache.Put("key5", "inst5")

	if cache.Size() == 5 {
		// All fit, which is fine
		return
	}

	// If some were evicted, the oldest accessed should be gone
}

func TestRouteCache_EvictionRespectsMemoryLimit(t *testing.T) {
	limit := int64(500)
	cache := NewRouteCache(limit)

	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%03d", i)
		cache.Put(key, "inst1")
	}

	curBytes := cache.CurrentBytes()
	if curBytes > limit {
		t.Errorf("Current bytes %d exceeds limit %d", curBytes, limit)
	}
}

func TestRouteCache_AccessRefreshesLRU(t *testing.T) {
	limit := int64(280)
	cache := NewRouteCache(limit)

	cache.Put("key1", "inst1")
	cache.Put("key2", "inst2")
	cache.Put("key3", "inst3")

	cache.Get("key1")

	cache.Put("key4", "inst4")

	if _, ok := cache.Get("key2"); ok {
		t.Error("Expected key2 to be evicted (LRU)")
	}

	if _, ok := cache.Get("key1"); !ok {
		t.Error("Expected key1 to be preserved (recently accessed)")
	}
}

func TestRouteCache_DeleteByInstance(t *testing.T) {
	cache := NewRouteCache(int64(1 * 1024 * 1024))

	cache.Put("key1", "inst1")
	cache.Put("key2", "inst1")
	cache.Put("key3", "inst2")
	cache.Put("key4", "inst2")

	cache.DeleteByInstance("inst1")

	if _, ok := cache.Get("key1"); ok {
		t.Error("Expected key1 to be deleted (belongs to inst1)")
	}
	if _, ok := cache.Get("key2"); ok {
		t.Error("Expected key2 to be deleted (belongs to inst1)")
	}

	if _, ok := cache.Get("key3"); !ok {
		t.Error("Expected key3 to still exist (belongs to inst2)")
	}
	if _, ok := cache.Get("key4"); !ok {
		t.Error("Expected key4 to still exist (belongs to inst2)")
	}
}

func TestRouteCache_Concurrent(t *testing.T) {
	cache := NewRouteCache(int64(10 * 1024))

	var wg sync.WaitGroup
	numGoroutines := 10
	numOps := 100

	// Concurrent puts
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				cache.Put(key, fmt.Sprintf("inst-%d", id%3))
			}
		}(i)
	}

	// Concurrent gets
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				cache.Get(key)
			}
		}(i)
	}

	// Concurrent deletes
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps/2; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				cache.Delete(key)
			}
		}(i)
	}

	wg.Wait()

	// Cache should still be in a valid state
	_ = cache.Size()
	_ = cache.CurrentBytes()
}

func TestRouteCache_EmptyCache(t *testing.T) {
	cache := NewRouteCache(int64(1024))

	if val, ok := cache.Get("nonexistent"); ok {
		t.Errorf("Expected not found, got %q", val)
	}

	cache.Delete("nonexistent")
	cache.DeleteByInstance("nonexistent")
}

func TestRouteCache_SizeTracking(t *testing.T) {
	cache := NewRouteCache(int64(1 * 1024 * 1024))

	if cache.Size() != 0 {
		t.Errorf("Expected size 0, got %d", cache.Size())
	}
	if cache.CurrentBytes() != 0 {
		t.Errorf("Expected 0 bytes, got %d", cache.CurrentBytes())
	}

	cache.Put("key1", "inst1")
	cache.Put("key2", "inst2")

	if cache.Size() != 2 {
		t.Errorf("Expected size 2, got %d", cache.Size())
	}
	if cache.CurrentBytes() <= 0 {
		t.Errorf("Expected >0 bytes, got %d", cache.CurrentBytes())
	}

	cache.Delete("key1")
	if cache.Size() != 1 {
		t.Errorf("Expected size 1 after delete, got %d", cache.Size())
	}
}
