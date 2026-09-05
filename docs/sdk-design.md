# KVCache SDK Design Document

## 1. Overview

The KVCache SDK is a distributed client library that enables application services to access multiple KVCache instances across multiple physical machines. The SDK provides:

- **Transparent key-instance mapping**: Applications don't need to track which instance stores which key
- **Local-affinity routing**: Prioritize writing to instances on the same physical machine
- **Capacity-aware selection**: Avoid instances with disk usage > 80%
- **Service discovery**: Automatic discovery of active KVCache instances via TiKV
- **LRU-optimized routing cache**: Memory-limited LRU cache for key-instance mappings to minimize TiKV access

## 2. Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Application Service                  │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│                      KVCache SDK                        │
│  ┌───────────────────────────────────────────────────┐ │
│  │           External API Layer (8 methods)           │ │
│  │   Set() Get() Delete() MSet() MGet() MDelete()    │ │
│  │              NewClient()  Close()                  │ │
│  └───────────────────────────────────────────────────┘ │
│  ┌───────────────────────────────────────────────────┐ │
│  │              Route Management Layer                │ │
│  │  ┌─────────────┐   ┌─────────────────────────┐   │ │
│  │  │ LRU Cache   │   │    Instance Picker      │   │ │
│  │  │ (1GB limit) │   │ (local affinity +       │   │ │
│  │  │  key→inst   │   │  capacity awareness)    │   │ │
│  │  └─────────────┘   └─────────────────────────┘   │ │
│  └───────────────────────────────────────────────────┘ │
│  ┌──────────────────┐  ┌────────────────────────────┐ │
│  │ Service Discovery│  │   gRPC Connection Pool     │ │
│  │ (TiKV watcher)   │  │   (per-instance pooling)   │ │
│  └──────────────────┘  └────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
                          │
            ┌─────────────┼─────────────┐
            ▼             ▼             ▼
      ┌──────────┐  ┌──────────┐  ┌──────────┐
      │  gRPC    │  │  gRPC    │  │  gRPC    │
      │Instance A│  │Instance B│  │Instance C│
      │(nodeA)   │  │(nodeA)   │  │(nodeB)   │
      └──────────┘  └──────────┘  └──────────┘
```

## 3. External API

### 3.1 Initialization

```go
sdk, err := client.NewClient(Config{
    Node:                "nodeA",
    TiKVPD:              "192.168.1.100:2379",
    RefreshInterval:     1 * time.Second,
    HeartbeatTimeout:    5 * time.Second,
    UsageThreshold:      0.80,
    RouteCacheSize:      1 * 1024 * 1024 * 1024, // 1GB
})
```

### 3.2 Core Methods

```go
// Single key operations
err := sdk.Set(ctx, "user:123", []byte("hello"))
value, err := sdk.Get(ctx, "user:123")
err := sdk.Delete(ctx, "user:123")

// Batch operations
err := sdk.MSet(ctx, map[string][]byte{
    "key1": []byte("val1"),
    "key2": []byte("val2"),
})
values, err := sdk.MGet(ctx, []string{"key1", "key2", "key3"})
err := sdk.MDelete(ctx, []string{"key1", "key2"})

// Lifecycle
sdk.Close()
```

### 3.3 Error Types

```go
var (
    ErrKeyNotFound      = errors.New("kvcache: key not found")
    ErrInstanceOffline  = errors.New("kvcache: instance offline")
    ErrNoInstances      = errors.New("kvcache: no instances available")
    ErrTiKVUnavailable  = errors.New("kvcache: TiKV unavailable")
)
```

## 4. Internal Components

### 4.1 Service Discovery

**TiKV Keyspace:**
```
/kvcache/instances/{name} → {
    "name": "a1",
    "node": "nodeA", 
    "addr": "192.168.1.10:33000",
    "capacity": 107374182400,
    "available": 75161927680,
    "last_heartbeat": 1234567890
}
```

**Discovery Flow:**
```go
func (c *Client) discoverInstances() {
    ticker := time.NewTicker(c.config.RefreshInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            instances := c.fetchFromTiKV("/kvcache/instances/")
            c.activeInstances.Store(filterActive(instances, c.config.HeartbeatTimeout))
        case <-c.ctx.Done():
            return
        }
    }
}
```

### 4.2 Instance Picker

Selects an instance for write operations based on:

1. **Local affinity**: Prefer instances on the same physical machine
2. **Capacity awareness**: Skip instances with > 80% disk usage
3. **Fallback**: If no local instances available, use any available instance

```go
func (c *Client) pickInstance() (*Instance, error) {
    instances := c.activeInstances.Load().([]*Instance)
    
    // Filter by capacity
    available := filter(instances, func(i *Instance) bool {
        return i.UsagePercent() < c.config.UsageThreshold
    })
    
    // Prefer local instances
    local := filter(available, func(i *Instance) bool {
        return i.Node == c.config.Node
    })
    if len(local) > 0 {
        return randomSelect(local), nil
    }
    
    // Fallback to any available instance
    if len(available) > 0 {
        return randomSelect(available), nil
    }
    
    // Last resort: use any instance even if overloaded
    any := filter(instances, func(i *Instance) bool {
        return i.Node == c.config.Node
    })
    if len(any) > 0 {
        return randomSelect(any), nil
    }
    
    return nil, ErrNoInstances
}
```

### 4.3 LRU Route Cache

In-memory LRU cache for key→instance mappings with memory limit.

```go
type RouteCache struct {
    maxSize  int64                    // Max bytes (default 1GB)
    capacity int                      // Max entries (for quick eviction)
    cache    map[string]*cacheEntry   // HashMap for O(1) lookup
    lru      list.List                // LRU list for eviction
    mutex    sync.RWMutex
}

type cacheEntry struct {
    key      string
    instance string
    elem     *list.Element  // Pointer to LRU list element
}

// Estimated memory per entry:
// - key: ~32 bytes (average)
// - instance: ~16 bytes (average)
// - overhead: ~32 bytes (map entry + list node + pointers)
// Total: ~80 bytes per entry
// 1GB can hold ~13 million entries
```

**Operations:**
```go
// Get: O(1) + move to front
func (c *RouteCache) Get(key string) (string, bool) {
    c.mutex.RLock()
    defer c.mutex.RUnlock()
    
    if entry, ok := c.cache[key]; ok {
        c.lru.MoveToFront(entry.elem)
        return entry.instance, true
    }
    return "", false
}

// Put: O(1) + evict if needed
func (c *RouteCache) Put(key, instance string) {
    c.mutex.Lock()
    defer c.mutex.Unlock()
    
    if entry, ok := c.cache[key]; ok {
        entry.instance = instance
        c.lru.MoveToFront(entry.elem)
        return
    }
    
    // Evict oldest entry if at capacity
    for c.currentMemory() >= c.maxSize && c.lru.Len() > 0 {
        c.evictOldest()
    }
    
    entry := &cacheEntry{key: key, instance: instance}
    entry.elem = c.lru.PushFront(entry)
    c.cache[key] = entry
}

// Delete: O(1)
func (c *RouteCache) Delete(key string) {
    c.mutex.Lock()
    defer c.mutex.Unlock()
    
    if entry, ok := c.cache[key]; ok {
        c.lru.Remove(entry.elem)
        delete(c.cache, key)
    }
}
```

### 4.4 TiKV Index Management

Maintains the key→instance mapping in TiKV for persistence.

```go
// TiKV keyspace for index
/kvcache/index/{key} → {
    "instance": "a1",
    "created_at": 1234567890,
    "updated_at": 1234567890
}

func (c *Client) writeIndex(key string, instanceName string) error {
    data := &IndexData{
        Instance:  instanceName,
        CreatedAt: time.Now().Unix(),
        UpdatedAt: time.Now().Unix(),
    }
    value, _ := json.Marshal(data)
    return c.tikvClient.Put([]byte("/kvcache/index/"+key), value)
}

func (c *Client) readIndex(key string) (string, error) {
    value, err := c.tikvClient.Get([]byte("/kvcache/index/" + key))
    if err != nil {
        return "", ErrKeyNotFound
    }
    
    var data IndexData
    json.Unmarshal(value, &data)
    return data.Instance, nil
}

func (c *Client) deleteIndex(key string) error {
    return c.tikvClient.Delete([]byte("/kvcache/index/" + key))
}
```

## 5. Core Flows

### 5.1 Set Flow

```
sdk.Set(ctx, key, value)
  │
  ├─ 1. Pick instance (local affinity + capacity check)
  │
  ├─ 2. Write to instance via gRPC
  │     └─ instance.Set(ctx, key, value)
  │
  ├─ 3. Write index to TiKV
  │     └─ PUT /kvcache/index/{key} = {instance: "a1", ...}
  │
  └─ 4. Update LRU cache
        └─ cache.Put(key, "a1")
```

### 5.2 Get Flow

```
sdk.Get(ctx, key)
  │
  ├─ 1. Try LRU cache first
  │     └─ cache.Get(key)
  │        ├─ HIT: instance = "a1", goto step 3
  │        └─ MISS: goto step 2
  │
  ├─ 2. Query TiKV index
  │     └─ GET /kvcache/index/{key}
  │        ├─ HIT: instance = "a1", cache.Put(key, instance)
  │        └─ MISS: return ErrKeyNotFound
  │
  └─ 3. Read from instance via gRPC
        └─ instance.Get(ctx, key)
           ├─ SUCCESS: return value
           └─ FAIL (ErrInstanceOffline):
              └─ Delete cache entry, return error
```

### 5.3 Delete Flow

```
sdk.Delete(ctx, key)
  │
  ├─ 1. Lookup instance (cache or TiKV)
  │     └─ instance = getInstanceForKey(key)
  │
  ├─ 2. Delete from instance via gRPC
  │     └─ instance.Delete(ctx, key)
  │
  ├─ 3. Delete index from TiKV
  │     └─ DELETE /kvcache/index/{key}
  │
  └─ 4. Delete from LRU cache
        └─ cache.Delete(key)
```

### 5.4 MSet Flow

```
sdk.MSet(ctx, keyValues map[string][]byte)
  │
  ├─ 1. Group keys by instance (using picker)
  │     └─ assignments: map[instance][]key
  │
  ├─ 2. Write to instances in parallel
  │     └─ for each instance: go instance.MSet(ctx, keys)
  │
  ├─ 3. Write indexes to TiKV
  │     └─ for each key: PUT /kvcache/index/{key}
  │
  └─ 4. Update LRU cache
        └─ for each key: cache.Put(key, instance)
```

### 5.5 MGet Flow

```
sdk.MGet(ctx, keys []string)
  │
  ├─ 1. Batch lookup instances (cache or TiKV)
  │     └─ assignments: map[instance][]key
  │        (keys not in cache/index are skipped)
  │
  ├─ 2. Read from instances in parallel
  │     └─ for each instance: go instance.MGet(ctx, keys)
  │
  └─ 3. Merge results and return
        └─ results: map[string][]byte
```

### 5.6 MDelete Flow

```
sdk.MDelete(ctx, keys []string)
  │
  ├─ 1. Batch lookup instances
  │     └─ assignments: map[instance][]key
  │
  ├─ 2. Delete from instances in parallel
  │     └─ for each instance: go instance.MDelete(ctx, keys)
  │
  ├─ 3. Delete indexes from TiKV
  │     └─ for each key: DELETE /kvcache/index/{key}
  │
  └─ 4. Delete from LRU cache
        └─ for each key: cache.Delete(key)
```

## 6. Connection Management

### 6.1 gRPC Connection Pool

Each instance maintains a connection pool for concurrent requests.

```go
type ConnectionPool struct {
    connections []*grpc.ClientConn
    index       int32
    mutex       sync.Mutex
}

func (p *ConnectionPool) Get() *grpc.ClientConn {
    i := atomic.AddInt32(&p.index, 1)
    return p.connections[i%int32(len(p.connections))]
}
```

### 6.2 Connection Lifecycle

- **Lazy initialization**: Create connection on first use of instance
- **Auto-reconnect**: Detect connection failure and reconnect
- **Cleanup**: Close all connections when instance is removed from active list

## 7. Error Handling

### 7.1 Instance Failure Detection

```
Instance failure scenarios:

1. Heartbeat timeout (5s):
   └─ Instance removed from active list
   └─ Related cache entries remain (may point to offline instance)

2. gRPC connection failure:
   └─ Detected during operation
   └─ Delete cache entry for affected keys
   └─ Next operation will query TiKV and fail with ErrInstanceOffline

3. TiKV unavailable:
   └─ Service discovery continues with last known list
   └─ Write operations fail with ErrTiKVUnavailable
   └─ Read operations can still work via cache (with stale data risk)
```

### 7.2 Error Response Strategy

```go
// Business logic error handling examples

value, err := sdk.Get(ctx, "user:123")
if err != nil {
    switch {
    case errors.Is(err, client.ErrKeyNotFound):
        // Key doesn't exist, return default value
        return defaultValue
    
    case errors.Is(err, client.ErrInstanceOffline):
        // Cache is stale or instance died, retry without cache
        value, err = sdk.GetWithoutCache(ctx, "user:123")
        if err != nil {
            // Data is lost, need to regenerate
            return regenerateData("user:123")
        }
        return value
    
    case errors.Is(err, client.ErrNoInstances):
        // No available instances, wait and retry
        time.Sleep(time.Second)
        return retry()
    
    case errors.Is(err, client.ErrTiKVUnavailable):
        // TiKV is down, but cache might have the mapping
        value, err = sdk.GetFromCacheOnly(ctx, "user:123")
        if err != nil {
            return err
        }
        return value
    }
}
```

## 8. Configuration

### 8.1 Client Config

```go
type Config struct {
    Node string              // Physical machine identifier (required)
    TiKVPD string            // TiKV PD address (required)
    
    // Tuning
    RefreshInterval time.Duration  // Service discovery interval (default: 1s)
    HeartbeatTimeout time.Duration // Instance heartbeat timeout (default: 5s)
    UsageThreshold float64         // Disk usage threshold (default: 0.80)
    RouteCacheSize int64           // Route cache memory limit in bytes (default: 1GB)
    
    // gRPC
    MaxRecvMsgSize int           // Max gRPC message size (default: 4MB)
    ConnPoolSize int             // Connections per instance (default: 3)
}
```

### 8.2 Tuning Guidelines

| Parameter | Impact | Recommendation |
|-----------|--------|----------------|
| RefreshInterval | Trade-off: freshness vs TiKV load | 1s for most cases, increase to 5s for large clusters |
| HeartbeatTimeout | Trade-off: failure detection speed vs false positives | 3x heartbeat interval (default 5s for 1s heartbeat) |
| UsageThreshold | Trade-off: capacity utilization vs performance headroom | 0.80-0.90 depending on workload |
| RouteCacheSize | Trade-off: hit rate vs memory usage | 1GB for ~13M keys, adjust based on key count |
| ConnPoolSize | Trade-off: throughput vs connection overhead | 3-5 for high-throughput workloads |

## 9. Performance Characteristics

### 9.1 Set Operation

```
Total time = 
    Instance selection: O(1) [in-memory]
  + gRPC write: O(network latency)
  + TiKV index write: O(network latency)
  + Cache update: O(1)

With cache optimization (write-through):
- First call: gRPC latency × 2 (instance + TiKV)
- Subsequent calls: Same (TiKV write required for every Set)
```

### 9.2 Get Operation

```
Cache HIT (most common):
Total time = 
    Cache lookup: O(1)
  + gRPC read: O(network latency)
  
Cache MISS (first call or after eviction):
Total time = 
    Cache lookup: O(1)
  + TiKV index read: O(network latency)
  + gRPC read: O(network latency)

Expected cache hit rate: >95% with 1GB cache and stable workload
```

### 9.3 MGet Operation

```
Batch lookup optimization:
- TiKV supports batch get (single request for multiple keys)
- Parallel gRPC reads to different instances

Total time = 
    Batch cache lookup: O(batch_size)
  + TiKV batch index read (for misses): O(1 RPC + O(batch_size) processing)
  + Parallel gRPC reads: O(network latency) [parallelized across instances]

Performance gain: 10-100x over sequential MGet calls
```

## 10. Limitations and Trade-offs

### 10.1 No Data Replication

Each key exists on exactly one instance. If the instance fails, the data is lost until the instance recovers.

**Mitigation**: Applications should treat KVCache as a cache layer and have a fallback (e.g., primary database).

### 10.2 No Automatic Rebalancing

Once a key is assigned to an instance, it stays there. New instances don't automatically receive existing keys.

**Mitigation**: For planned scaling, use key prefix patterns to distribute keys evenly (e.g., `user:${i%num_instances}:123`).

### 10.3 Cache-TiKV Consistency

LRU cache and TiKV index can temporarily be inconsistent (until next refresh or eviction).

**Impact**: Read operations may hit stale cache entries pointing to failed instances. Next operation will detect and correct.

### 10.4 TiKV Dependency

If TiKV is unavailable, new writes fail and reads are limited to cached keys.

**Mitigation**: Deploy TiKV in HA mode. Consider local file backup for critical index data in future versions.

## 11. Future Enhancements

- [ ] **Consistent Hashing**: Replace random selection with consistent hashing for better load distribution
- [ ] **Automatic Rebalancing**: Migrate keys from overloaded instances to underloaded ones
- [ ] **Read Replicas**: Support read-only replicas for high-read workloads
- [ ] **Local Index Backup**: Periodically write TiKV index to local file for TiKV failure recovery
- [ ] **Metrics Export**: Export SDK metrics to Prometheus for monitoring
