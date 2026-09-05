package client

import (
	"container/list"
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	routeCacheMetricsRegistered sync.Once
	routeCacheHits              = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "kvcache_sdk",
		Subsystem: "route_cache",
		Name:      "hits_total",
		Help:      "Total number of route cache hits",
	})
	routeCacheMisses = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "kvcache_sdk",
		Subsystem: "route_cache",
		Name:      "misses_total",
		Help:      "Total number of route cache misses",
	})
	routeCacheSize = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "kvcache_sdk",
		Subsystem: "route_cache",
		Name:      "entries",
		Help:      "Current number of entries in route cache",
	}, nil)
	routeCacheBytes = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "kvcache_sdk",
		Subsystem: "route_cache",
		Name:      "size_bytes",
		Help:      "Current size of route cache in bytes",
	}, nil)
)

type routeEntry struct {
	key       string
	instance  string
	elem      *list.Element
	sizeBytes int64
}

type RouteCache struct {
	maxSize      int64
	currentBytes int64
	cache        map[string]*routeEntry
	lru          list.List
	reverseIdx   map[string]map[string]struct{}
	mu           sync.RWMutex
}

func NewRouteCache(maxSizeBytes int64) *RouteCache {
	rc := &RouteCache{
		maxSize:    maxSizeBytes,
		cache:      make(map[string]*routeEntry),
		reverseIdx: make(map[string]map[string]struct{}),
	}

	// 注册 metrics（只注册一次，否则同进程创建第二个 RouteCache 会 MustRegister panic）。
	// 注意：GaugeFunc 闭包捕获的是首个实例，多实例时 gauge 只反映第一个 cache。
	routeCacheMetricsRegistered.Do(func() {
		prometheus.MustRegister(routeCacheHits)
		prometheus.MustRegister(routeCacheMisses)
		routeCacheSize = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "kvcache_sdk",
			Subsystem: "route_cache",
			Name:      "entries",
			Help:      "Current number of entries in route cache",
		}, func() float64 {
			return float64(rc.Size())
		})
		routeCacheBytes = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "kvcache_sdk",
			Subsystem: "route_cache",
			Name:      "size_bytes",
			Help:      "Current size of route cache in bytes",
		}, func() float64 {
			return float64(rc.CurrentBytes())
		})
		prometheus.MustRegister(routeCacheSize)
		prometheus.MustRegister(routeCacheBytes)
	})

	return rc
}

func estimateSize(key, instance string) int64 {
	return int64(len(key))+int64(len(instance))+80
}
func (rc *RouteCache) Get(key string) (string, bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if entry, ok := rc.cache[key]; ok {
		rc.lru.MoveToFront(entry.elem)
		routeCacheHits.Inc()
		return entry.instance, true
	}
	routeCacheMisses.Inc()
	return "", false
}

func (rc *RouteCache) Put(key, instance string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if existing, ok := rc.cache[key]; ok {
		oldSize := existing.sizeBytes
		newSize := estimateSize(key, instance)

		existing.instance = instance
		existing.sizeBytes = newSize
		rc.lru.MoveToFront(existing.elem)

		atomic.AddInt64(&rc.currentBytes, newSize-oldSize)
		return
	}

	size := estimateSize(key, instance)

	for rc.currentBytes+size > rc.maxSize && rc.lru.Len() > 0 {
		rc.evictOldest()
	}

	entry := &routeEntry{
		key:       key,
		instance:  instance,
		sizeBytes: size,
	}
	entry.elem = rc.lru.PushFront(entry)
	rc.cache[key] = entry
	atomic.AddInt64(&rc.currentBytes, size)

	if rc.reverseIdx[instance] == nil {
		rc.reverseIdx[instance] = make(map[string]struct{})
	}
	rc.reverseIdx[instance][key] = struct{}{}
}

func (rc *RouteCache) Delete(key string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	entry, ok := rc.cache[key]
	if !ok {
		return
	}

	if keys, ok := rc.reverseIdx[entry.instance]; ok {
		delete(keys, key)
		if len(keys) == 0 {
			delete(rc.reverseIdx, entry.instance)
		}
	}

	rc.lru.Remove(entry.elem)
	delete(rc.cache, key)
	atomic.AddInt64(&rc.currentBytes, -entry.sizeBytes)
}

func (rc *RouteCache) evictOldest() {
	back := rc.lru.Back()
	if back == nil {
		return
	}

	entry := back.Value.(*routeEntry)

	if keys, ok := rc.reverseIdx[entry.instance]; ok {
		delete(keys, entry.key)
		if len(keys) == 0 {
			delete(rc.reverseIdx, entry.instance)
		}
	}

	rc.lru.Remove(back)
	delete(rc.cache, entry.key)
	atomic.AddInt64(&rc.currentBytes, -entry.sizeBytes)
}

func (rc *RouteCache) DeleteByInstance(instance string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	keys, ok := rc.reverseIdx[instance]
	if !ok {
		return
	}

	for key := range keys {
		if entry, ok := rc.cache[key]; ok {
			rc.lru.Remove(entry.elem)
			delete(rc.cache, key)
			atomic.AddInt64(&rc.currentBytes, -entry.sizeBytes)
		}
	}
	delete(rc.reverseIdx, instance)
}

func (rc *RouteCache) Size() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.lru.Len()
}

func (rc *RouteCache) CurrentBytes() int64 {
	return atomic.LoadInt64(&rc.currentBytes)
}
