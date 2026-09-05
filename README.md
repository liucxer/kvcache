# KVCache - High Performance Key-Value Storage Service

## Introduction

KVCache is a high-performance key-value storage service built in Go, using RocksDB as the underlying storage engine. It exposes both gRPC and HTTP interfaces, supporting large-scale data storage with fast access.

## Features

- **High Performance Storage**: Built on RocksDB engine for efficient key-value storage and retrieval
- **Dual Protocol**: Provides both gRPC and HTTP RESTful interfaces
- **Large Value Storage**: Automatically stores values >1MB to separate disk files; RocksDB retains only path references
- **Memory Cache**: `sync.Map`-based L1 read cache with auto-fill for small values (<10KB)
- **Batch Operations**: Atomic batch Set, Get, and Delete via RocksDB WriteBatch
- **Prefix Scan**: Ordered prefix-based scanning over RocksDB iterators
- **Auto Eviction**: FIFO eviction triggered by disk usage threshold
- **Runtime Configuration**: Modify service configuration via API at runtime
- **Prometheus Monitoring**: End-to-end operation counters, error rates, and latency histograms
- **Multi-Instance Deployment**: Automatic port discovery and process management scripts
- **Client SDK**: Multi-backend round-robin load balancing with failover retry
- **Web Management UI**: Graphical KV CRUD, scan, and configuration management

## Technology Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.18+ |
| Storage Engine | RocksDB (via grocksdb CGO binding) |
| RPC Framework | gRPC + Protocol Buffers |
| HTTP Framework | Gin |
| Monitoring | Prometheus client_golang |

## Architecture

```
main.go (auto port discovery 33000-33100)
  ├── config/          → Config structs + defaults (pure data, no I/O)
  ├── storage/         → RocksDB wrapper + disk large-value store + eviction engine
  │     └── Storage interface
  ├── service/         → Business logic + Prometheus metrics + memory cache
  │     └── depends on storage.Storage
  ├── api/             → gRPC Server + HTTP Server (thin adapter layer)
  │     └── depends on service.KVService
  ├── client/          → gRPC client SDK (round-robin + failover retry)
  │     └── depends on proto
  └── web/             → Static frontend (Tailwind CSS SPA)
```

Dependency chain (unidirectional): `api → service → storage`. `main.go` wires everything together.

## Project Structure

```
├── api/                          # API layer
│   ├── grpc_server.go            #   gRPC adapter (implements proto interfaces)
│   └── http_server.go            #   HTTP adapter (Gin routes)
├── client/                       # Client SDK
│   ├── client.go                 #   Multi-backend round-robin + retry logic
│   └── example.go                #   Usage example
├── config/                       # Configuration module
│   ├── config.go                 #   Config struct + DefaultConfig()
│   └── config_test.go            #   Config tests
├── proto/                        # Protocol Buffers
│   ├── kv.proto                  #   Service definition source
│   ├── kv.pb.go                  #   Generated message code
│   └── kv_grpc.pb.go            #   Generated gRPC stubs
├── service/                      # Business logic layer
│   ├── kv_service.go             #   KVService (cache + metrics + forwarding)
│   ├── metrics.go                #   Prometheus metric definitions
│   ├── service_test.go           #   Service unit tests
│   └── performance_test.go       #   Benchmarks
├── storage/                      # Storage layer
│   ├── storage.go                #   Storage interface definition
│   ├── rocksdb.go                #   RocksDB implementation (core)
│   ├── disk_store.go             #   Large value disk storage
│   ├── eviction.go               #   Eviction manager
│   └── storage_test.go           #   Storage unit tests
├── test/api/                     # Integration tests
│   ├── http_test.go              #   HTTP API tests + global setup
│   ├── grpc_test.go              #   gRPC API tests
│   └── grpc_performance_test.go  #   gRPC benchmarks
├── web/                          # Web frontend
│   ├── index.html                #   Single-page management UI
│   └── script.js                 #   Frontend interaction logic
├── main.go                       # Entry point
├── start-instances.sh            # Launch multiple instances
├── stop-instances.sh             # Stop instances
├── status-instances.sh           # Check instance status
├── Makefile                      # Build and test scripts
├── go.mod                        # Go module definition
└── go.sum                        # Dependency lock file
```

## Quick Start

### Prerequisites

- Go 1.18 or later
- GCC 4.8 or later (RocksDB CGO dependency)
- RocksDB development library (`librocksdb-dev` or equivalent)

### Install Dependencies

```bash
git clone https://github.com/yourusername/kvcache.git
cd kvcache
go mod download
```

### Build and Run

#### Using Makefile (Recommended)

```bash
make build    # Build
make run      # Build and run
make clean    # Clean build artifacts
```

#### Using Go Commands

```bash
# macOS (Homebrew)
CGO_CFLAGS="-I/opt/homebrew/include" \
CGO_LDFLAGS="-L/opt/homebrew/lib -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd" \
  go build -o kvcache .

# Linux
CGO_CFLAGS="-I/usr/include" \
CGO_LDFLAGS="-L/lib64 -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd" \
  go build -o kvcache .

./kvcache
```

### Port Allocation

On startup, the service allocates available port pairs via **automatic port discovery**:
- Scans range `33000-33100`
- **Even ports** assigned to gRPC, **odd ports** assigned to HTTP (e.g., 33000/33001, 33002/33003)
- Allocated ports are logged at startup

## Feature Details

### 1. Basic KV Operations (Set / Get / Delete)

**Write Path (Set)**:

1. Client sends HTTP `POST /api/v1/set` with `{key, value, ttl}` or gRPC `Set` RPC with `{key: bytes, value: bytes}`
2. `api` layer validates non-empty key, forwards to `service.KVService.Set()`
3. `KVService` calls `storage.Set()` to write to RocksDB, recording latency and counter metrics
4. `RocksDBStorage.Set()` branches on value size:
   - **Small value** (≤ `DiskThreshold`, default 1MB): `PutCF()` directly into RocksDB
   - **Large value** (> threshold): write to a file under `value_data/`, store path reference `"__rocksdb_disk_store__://filename"` in RocksDB
5. Back in `KVService`, if cache is enabled and value < `SizeThreshold` (10KB), store in memory cache

**Read Path (Get)**:

1. HTTP `GET /api/v1/get/:key` or gRPC `Get` RPC
2. `KVService.Get()` checks `sync.Map` cache first:
   - **Cache hit**: return immediately
   - **Cache miss**: fall through to `storage.Get()`
3. `RocksDBStorage.Get()` inspects the stored value:
   - Empty → `found=false`
   - `"__evicted__"` → return "value has been evicted" error
   - Starts with `__rocksdb_disk_store__://` → load full data from `DiskStore`
   - Normal value → copy and return (avoids RocksDB C-layer buffer invalidation)
4. After read, if value < cache threshold, **fill it into cache** (cache-aside with read-through)

**Delete Path (Delete)**:

1. HTTP `DELETE /api/v1/delete/:key` or gRPC `Delete` RPC
2. `storage.Delete()` reads RocksDB to check type: for large values, deletes disk file first, then removes the key from RocksDB
3. `KVService.Delete()` removes the key from the memory cache; key count gauge decremented

### 2. Batch Operations (MSet / MGet / MDelete)

**MSet** — Atomic batch write:
- HTTP `POST /api/v1/mset` with `{kvs: {k1: v1, ...}, ttl}`
- `storage.MSet()` uses RocksDB **WriteBatch** — all puts packaged into a single atomic commit
- Large values are still routed to disk files; small values are batch-written to cache

**MGet** — Batch read with cache penetration optimization:
- `KVService.MGet()` scans all keys against `sync.Map`; hits go straight to the result set
- Missed keys are collected and sent to `storage.MGet()` in one call
- Returned values below the threshold are filled back into cache

**MDelete** — Batch delete:
- Uses WriteBatch for atomic deletion (each key is checked for disk file references first)
- Keys are individually removed from the memory cache

### 3. Prefix Scan (Scan)

Implemented on top of RocksDB ordered iterators.

- gRPC offers `ScanKeys` (returns key list only) and `ScanKeyValues` (returns key→value map)
- HTTP `GET /api/v1/scan?prefix=xxx&limit=100` returns a key-value map
- `ScanWithValues()` uses `Seek(prefix)` to jump to the prefix start, `break`s on the first non-matching key (leverages sorted order, efficient)
- `Scan()` uses `SeekToFirst()` to iterate everything and filters by prefix (less efficient)
- Both skip the reserved system key `global.config`

### 4. Large Value Disk Store (DiskStore)

Secondary storage for values exceeding 1MB:

| Operation | Implementation |
|-----------|---------------|
| Store | SHA256(data) as filename, written to `value_data/`; content-addressable (identical content auto-deduplicated) |
| Load | `os.ReadFile` by filename |
| Delete | `os.Remove`; file-not-found silently ignored |

Reference link: RocksDB stores `"__rocksdb_disk_store__://filename"`; reads detect the prefix and redirect to disk.

### 5. Memory Cache

`sync.Map`-based L1 read cache in `KVService`:

- **Enabled when**: `config.cache.enabled = true` (default: on)
- **Threshold**: `config.cache.size_threshold = 10240` (10KB)
- **Write triggers**: Set success → fill cache; Get/MGet cache miss → fill back
- **Invalidation triggers**: Delete/MDelete → remove from cache
- **Concurrency**: `sync.Map` is inherently safe for concurrent reads/writes

### 6. Auto Eviction (Eviction)

Design goal: When disk usage exceeds the configured threshold, evict large values FIFO (oldest creation time first).

| Config | Default |
|--------|---------|
| `eviction.enabled` | true |
| `eviction.disk_usage_threshold` | 80% |
| `eviction.check_interval` | 60 seconds |
| `eviction.batch_size` | max 100 keys per round |

**Eviction flow**:
1. `EvictionManager` periodically runs `checkAndEvict()`
2. Once threshold is exceeded, iterates the creation-time index; for each large-value key:
   - Delete the disk file
   - Replace the RocksDB value with `"__evicted__"` marker
   - Remove from creation-time index
3. A Get on an evicted key returns `"value has been evicted"` error

### 7. Runtime Configuration Management

Configuration is set via `DefaultConfig()` in code and can be modified at runtime through the API.

| Block | Field | Default |
|-------|-------|---------|
| `rocksdb` | `path` | `./data` |
| `rocksdb` | `block_cache_size` | 64 MB |
| `value` | `disk_threshold` | 1 MB (1048576 bytes) |
| `value` | `disk_path` | `./value_data` |
| `eviction` | `enabled` | true |
| `eviction` | `disk_usage_threshold` | 0.8 (80%) |
| `eviction` | `check_interval` | 60 seconds |
| `eviction` | `batch_size` | 100 |
| `cache` | `enabled` | true |
| `cache` | `size_threshold` | 10240 bytes (10KB) |

**Update methods**:
- HTTP `POST /api/v1/config`: Flat JSON fields (e.g., `rocksdb_path`, `max_disk_usage`)
- gRPC `UpdateConfig`: Nested JSON
- Web frontend configuration tab

When eviction config changes, the `EvictionManager` is automatically restarted.

### 8. Prometheus Monitoring

All metrics use `cachefs` as the namespace, exposed via `/metrics`.

**Operation Counters**:
```
cachefs_kv_sets_total / gets_total / deletes_total / scans_total
cachefs_kv_msets_total / mgets_total / mdeletes_total
cachefs_config_updates_total
cachefs_health_checks_total
```

**Error Counters (CounterVec, split by error label)**:
```
cachefs_kv_set_errors_total{error="empty_key"}
cachefs_kv_get_errors_total{error="not_found"}
... one counter per operation type
```

**Latency Histograms (HistogramVec, in seconds)**:
```
cachefs_kv_set_latency_seconds{type="kv"}
cachefs_kv_get_latency_seconds{type="config"}
... buckets: ExponentialBuckets(0.001, 2, 10) i.e. 1ms ~ 512ms
```

**Status Gauges**:
```
cachefs_kv_keys_current        # total key count
cachefs_storage_disk_usage_bytes
cachefs_storage_memory_usage_bytes
```

## API Reference

### HTTP API (Gin Framework)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/set` | Set key-value pair `{key, value, ttl}` |
| GET | `/api/v1/get/:key` | Get value |
| DELETE | `/api/v1/delete/:key` | Delete key-value pair |
| GET | `/api/v1/scan?prefix=&limit=` | Prefix scan |
| POST | `/api/v1/mset` | Batch set `{kvs: {...}, ttl}` |
| POST | `/api/v1/mget` | Batch get `{keys: [...]}` |
| POST | `/api/v1/mdelete` | Batch delete `{keys: [...]}` |
| GET | `/api/v1/config` | Get current configuration |
| POST | `/api/v1/config` | Update configuration |
| GET | `/health` | Health check |
| GET | `/metrics` | Prometheus metrics |
| GET | `/` | Redirect to `/web/index.html` |
| GET | `/web/*` | Web UI static files |

#### Request/Response Examples

**Set**:
```json
// POST /api/v1/set
{"key": "example", "value": "hello world", "ttl": 0}
// → {"success": true, "message": "key set successfully"}
```

**Get**:
```json
// GET /api/v1/get/example
// → {"key": "example", "value": "hello world"}
```

**Scan**:
```json
// GET /api/v1/scan?prefix=user&limit=100
// → {"prefix": "user", "limit": 100, "count": 2, "results": {"user1": "v1", "user2": "v2"}}
```

### gRPC API

Defined in `proto/kv.proto`, two services:

**KeyValueService** (10 RPCs):

| RPC | Description | Request | Response |
|-----|-------------|---------|----------|
| `Set` | Set key-value pair | `SetRequest{key, value}` | `SetResponse{success, error}` |
| `Get` | Get value | `GetRequest{key}` | `GetResponse{value, found, error}` |
| `Delete` | Delete key-value pair | `DeleteRequest{key}` | `DeleteResponse{success, error}` |
| `ScanKeys` | Scan keys | `ScanRequest{prefix}` | `ScanKeysResponse{keys}` |
| `ScanKeyValues` | Scan key-value pairs | `ScanRequest{prefix}` | `ScanKeyValuesResponse{key_values}` |
| `MSet` | Batch set | `MSetRequest{key_values}` | `MSetResponse{success, error}` |
| `MGet` | Batch get | `MGetRequest{keys}` | `MGetResponse{key_values}` |
| `MDelete` | Batch delete | `MDeleteRequest{keys}` | `MDeleteResponse{success, error}` |
| `GetConfig` | Get configuration | `GetConfigRequest{}` | `GetConfigResponse{config}` |
| `UpdateConfig` | Update configuration | `UpdateConfigRequest{config}` | `UpdateConfigResponse{success, error}` |

**Health** (1 RPC):

| RPC | Description |
|-----|-------------|
| `Check` | Health check; returns UNKNOWN / SERVING / NOT_SERVING / SERVICE_UNKNOWN |

Both key and value fields are `bytes` in the proto definition.

**HTTP vs gRPC differences**:
- HTTP Set accepts TTL (seconds); gRPC proto has no TTL field (hardcoded to 0)
- HTTP Scan returns a key-value map; gRPC can separately request key list or full mapping

## Client SDK

The `client/` package provides a gRPC client with multi-backend load balancing:

```go
serverAddrs := []string{
    "localhost:33000", "localhost:33002", "localhost:33004",
}
client, _ := client.NewClient(serverAddrs)

// Round-robin load balancing + automatic failover retry
client.Set(ctx, "key", []byte("value"), 0)
value, _ := client.Get(ctx, "key")
client.Delete(ctx, "key")
```

- **Load Balancing**: Round-robin across backends on each request
- **Failover Retry**: If one backend fails, automatically tries the next

## Web Management UI

The `web/` directory contains a single-page management interface (Tailwind CSS + vanilla JS):

| Tab | Function |
|-----|----------|
| Overview | Health check, monitoring metrics |
| Single Ops | Set (with TTL), Get, Delete forms |
| Batch Ops | MSet / MGet / MDelete with dynamic key-value row add/remove |
| Scan | Prefix search; toggle between keys-only or keys+values view |
| Config | Display current JSON config; form-based update |

The root path `/` redirects to `/web/index.html`.

## Multi-Instance Deployment

### Port Allocation

`main.go` scans the `33000-33100` range for available port pairs on startup (even=gRPC, odd=HTTP). Each instance gets a non-conflicting pair automatically.

### Instance Management Scripts

```bash
# Start instances (one per data directory)
./start-instances.sh /data/inst1 /data/inst2 /data/inst3

# Check instance status (ports, processes, logs)
./status-instances.sh

# Stop instances
./stop-instances.sh /data/inst1 /data/inst2
./stop-instances.sh all    # stop all running instances
```

Each instance maintains independently in its own directory:
- `data/` — RocksDB data
- `value_data/` — Large value disk store
- `{name}.log` — runtime log
- `{name}.pid` — process PID file

## Testing

### Run Tests

```bash
make test                 # All unit + integration tests
make test-config          # config package
make test-service         # service package
make test-storage         # storage package
make test-api             # Integration tests (HTTP + gRPC)
make test-http            # HTTP API tests only
make test-grpc            # gRPC API tests only
make test-verbose         # Verbose output
```

### Benchmarks

```bash
make test-performance             # All service-layer benchmarks
make test-benchmark-set           # Single-thread Set benchmark
make test-benchmark-get           # Single-thread Get benchmark
make test-benchmark-concurrent    # Concurrent benchmark
make test-benchmark-mixed         # Mixed ops (80% read + 20% write)
make test-grpc-performance        # All gRPC-layer benchmarks
make test-grpc-benchmark-set/get/concurrent/mixed
```

### Test Coverage

```bash
make test-coverage          # Full test coverage
make test-coverage-html     # Generate HTML coverage report
make test-coverage-storage  # storage package coverage
```

### Benchmark Results

#### Service Layer

| Test | Ops/sec | Avg Latency/op |
|------|---------|----------------|
| Set (single-thread) | ~316,732 | ~3,762 ns |
| Get (single-thread) | ~1,629,542 | ~783 ns |
| Set (concurrent) | ~214,846 | ~6,049 ns |
| Get (concurrent) | ~4,086,961 | ~309.7 ns |
| Mixed | ~747,752 | ~1,459 ns |

#### gRPC Client

| Test | Ops/sec | Avg Latency/op |
|------|---------|----------------|
| Set (single-thread) | ~20,506 | ~58,621 ns |
| Get (single-thread) | ~21,990 | ~49,830 ns |
| Set (concurrent) | ~65,038 | ~21,667 ns |
| Get (concurrent) | ~95,284 | ~15,663 ns |
| Mixed | ~76,860 | ~15,564 ns |

## Deployment Recommendations

1. **Data Directory**: Ensure sufficient disk space for RocksDB; SSD recommended for best performance
2. **Block Device Readahead**: At startup the server automatically sets `read_ahead_kb` on the block device backing `--value-dir` (default target: 4096 KB; flag `--readahead-kb`, `0` disables). This is critical for sequential read throughput of large values on NVMe — the default kernel value of 128 KB can cost ~25% read bandwidth. Requires write permission to `/sys/block/*/queue/read_ahead_kb` (run as root or pre-tune manually); failures are logged as warnings and do not block startup.
3. **Memory**: Tune system memory based on dataset size; RocksDB uses part of it as BlockCache (default 64MB)
3. **Networking**: Use a load balancer to distribute traffic across multiple instances
4. **Monitoring**: Scrape `/metrics` with Prometheus; pair with Grafana dashboards; set disk usage alerts
5. **Backup**: Periodically back up the RocksDB data directory, or use RocksDB Checkpoint for incremental backups

## FAQ

### Service Fails to Start
**Likely causes**: RocksDB library not installed, ports occupied, data directory permission denied
**Fix**: Verify GCC and librocksdb versions; ensure ports 33000-33100 are free

### Performance Degradation
**Likely causes**: Dataset too large, insufficient memory, disk I/O bottleneck
**Fix**: Use `make test-coverage` or benchmarks to locate the bottleneck; tune RocksDB BlockCache size; shard across multiple instances

### Connection Timeout
**Likely causes**: Network latency, high server load, oversized batch operations
**Fix**: Increase instance count; reduce the number of keys per batch operation

## License

MIT License
