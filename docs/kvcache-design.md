# KVCache Server Design Document

## 1. Overview

KVCache is a high-performance key-value storage service built on RocksDB. In distributed deployment mode, multiple KVCache instances run across multiple physical machines, coordinated by TiKV for service discovery and health monitoring.

**Key Features:**
- **Distributed deployment**: Multiple instances across multiple physical machines
- **Service registration**: Automatic registration to TiKV on startup
- **Health monitoring**: 1-second heartbeat with capacity reporting
- **Graceful shutdown**: Automatic deregistration on stop
- **Capacity reporting**: Real-time disk capacity and usage tracking

## 2. Architecture

### 2.1 Single Instance Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    KVCache Instance                     │
├─────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │
│  │  gRPC Server │  │  HTTP Server │  │  Web UI      │ │
│  │  :33000      │  │  :33001      │  │  :33002      │ │
│  └──────┬───────┘  └──────┬───────┘  └──────────────┘ │
│         └──────────────────┘                           │
│                    │                                   │
│  ┌─────────────────▼─────────────────────────────────┐ │
│  │              Service Layer                        │ │
│  │  - Business logic                                 │ │
│  │  - Prometheus metrics                             │ │
│  │  - Memory cache (sync.Map)                        │ │
│  └─────────────────┬─────────────────────────────────┘ │
│                    │                                   │
│  ┌─────────────────▼─────────────────────────────────┐ │
│  │              Storage Layer                        │ │
│  │  ┌──────────────┐  ┌──────────────┐              │ │
│  │  │   RocksDB    │  │  Disk Store  │              │ │
│  │  │   (data/)    │  │ (value_data) │              │ │
│  │  │              │  │  >1MB values │              │ │
│  │  └──────────────┘  └──────────────┘              │ │
│  │  ┌──────────────┐                                 │ │
│  │  │  Eviction    │                                 │ │
│  │  │  Manager     │                                 │ │
│  │  └──────────────┘                                 │ │
│  └───────────────────────────────────────────────────┘ │
│                                                        │
│  ┌───────────────────────────────────────────────────┐ │
│  │         TiKV Service Registration (new)           │ │
│  │  - Register on startup                            │ │
│  │  - Heartbeat every 1s                             │ │
│  │  - Report capacity                                │ │
│  │  - Deregister on shutdown                         │ │
│  └───────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

### 2.2 Distributed Deployment

```
                    ┌────────────────────────┐
                    │         TiKV           │
                    │  (Service Registry)    │
                    │                        │
                    │ /kvcache/instances/*   │
                    │   ├─ a1 (nodeA)        │
                    │   ├─ a2 (nodeA)        │
                    │   ├─ b1 (nodeB)        │
                    │   ├─ b2 (nodeB)        │
                    │   ├─ c1 (nodeC)        │
                    │   └─ c2 (nodeC)        │
                    └─────┬──────┬───────────┘
                          │      │
         ┌────────────────┘      └────────────────┐
         │                                        │
┌────────▼─────────┐                    ┌─────────▼────────┐
│  Physical Node A │                    │  Physical Node B │
│                  │                    │                  │
│  ┌────────────┐  │                    │  ┌────────────┐  │
│  │  App       │  │                    │  │  App       │  │
│  │  + SDK     │  │                    │  │  + SDK     │  │
│  └────────────┘  │                    │  └────────────┘  │
│  ┌────────────┐  │                    │  ┌────────────┐  │
│  │ kvcache-a1 │  │                    │  │ kvcache-b1 │  │
│  │ :33000     │  │                    │  │ :33000     │  │
│  └────────────┘  │                    │  └────────────┘  │
│  ┌────────────┐  │                    │  ┌────────────┐  │
│  │ kvcache-a2 │  │                    │  │ kvcache-b2 │  │
│  │ :33002     │  │                    │  │ :33002     │  │
│  └────────────┘  │                    │  └────────────┘  │
└──────────────────┘                    └──────────────────┘
```

## 3. Startup Parameters

### 3.1 New Distributed Parameters

```bash
./kvcache \
  --name=a1 \                      # Unique instance name (required)
  --node=nodeA \                   # Physical machine identifier (required)
  --addr=192.168.1.10:33000 \      # gRPC bind address (required)
  --tikv-pd=192.168.1.100:2379 \   # TiKV PD address (required for distributed mode)
  --data-dir=./data \              # RocksDB data directory
  --value-dir=./value_data         # Large value storage directory
```

### 3.2 Parameter Validation

```go
func validateConfig(cfg *Config) error {
    if cfg.InstanceName == "" {
        return errors.New("--name is required")
    }
    if cfg.NodeName == "" {
        return errors.New("--node is required")
    }
    if cfg.GRPCAddr == "" {
        return errors.New("--addr is required")
    }
    if cfg.TiKVPD == "" {
        return errors.New("--tikv-pd is required for distributed mode")
    }
    return nil
}
```

## 4. Service Registration

### 4.1 Registration Data Structure

```go
type InstanceInfo struct {
    Name        string `json:"name"`
    Node        string `json:"node"`
    Addr        string `json:"addr"`
    Capacity    int64  `json:"capacity"`    // Total disk bytes
    Available   int64  `json:"available"`   // Free disk bytes
    Used        int64  `json:"used"`        // Used disk bytes
    StartTime   int64  `json:"start_time"`  // Unix timestamp
}

// TiKV key: /kvcache/instances/{name}
// TiKV value: JSON-encoded InstanceInfo
```

### 4.2 Registration Flow

```
Startup sequence:

1. Parse command-line arguments
   └─ name, node, addr, tikv-pd, data-dir, value-dir

2. Initialize RocksDB storage
   └─ Open database with data directory
   └─ Create disk store directory for large values

3. Get initial disk capacity
   └─ capacity, available = getDiskCapacity(dataDir)
   └─ used = capacity - available

4. Connect to TiKV
   └─ client, err := tikvclient.Connect(tikvPD)

5. Register instance to TiKV
   └─ key = "/kvcache/instances/" + name
   └─ value = InstanceInfo{
         Name: name,
         Node: node,
         Addr: addr,
         Capacity: capacity,
         Available: available,
         Used: used,
         StartTime: time.Now().Unix(),
       }
   └─ client.Put(key, value)

6. Start heartbeat goroutine

7. Start gRPC server

8. Start HTTP server

9. Start eviction manager (if enabled)
```

## 5. Heartbeat Mechanism

### 5.1 Heartbeat Configuration

```go
const (
    HeartbeatInterval = 1 * time.Second  // Heartbeat frequency
    HeartbeatTimeout  = 5 * time.Second  // Timeout threshold
)
```

### 5.2 Heartbeat Flow

```go
func (s *Server) startHeartbeat() {
    ticker := time.NewTicker(HeartbeatInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            s.sendHeartbeat()
        case <-s.ctx.Done():
            return
        }
    }
}

func (s *Server) sendHeartbeat() {
    // Get current disk capacity
    capacity, available := getDiskCapacity(s.config.DataDir)
    used := capacity - available
    
    // Update instance info
    info := &InstanceInfo{
        Name:      s.config.InstanceName,
        Node:      s.config.NodeName,
        Addr:      s.config.GRPCAddr,
        Capacity:  capacity,
        Available: available,
        Used:      used,
        StartTime: s.startTime.Unix(),
    }
    
    // Write to TiKV
    key := "/kvcache/instances/" + s.config.InstanceName
    value, _ := json.Marshal(info)
    s.tikvClient.Put(key, value)
}
```

### 5.3 Disk Capacity Measurement

```go
import "syscall"

func getDiskCapacity(path string) (capacity, available int64) {
    var stat syscall.Statfs_t
    err := syscall.Statfs(path, &stat)
    if err != nil {
        return 0, 0
    }
    
    capacity = int64(stat.Blocks) * int64(stat.Bsize)
    available = int64(stat.Bavail) * int64(stat.Bsize)  // Available to non-root
    
    return capacity, available
}
```

**Note**: Uses `Bavail` instead of `Bfree` to get space available to non-root users, which is more accurate for capacity planning.

## 6. Graceful Shutdown

### 6.1 Shutdown Sequence

```
Shutdown signal (SIGINT/SIGTERM):

1. Stop accepting new requests
   └─ gRPC server: GracefulStop()
   └─ HTTP server: Shutdown(ctx)

2. Wait for in-flight requests (5s timeout)
   └─ Drain active connections

3. Stop heartbeat goroutine
   └─ Cancel context

4. Stop eviction manager
   └─ Stop periodic checks

5. Deregister from TiKV
   └─ DELETE /kvcache/instances/{name}

6. Close storage
   └─ RocksDB.Close()
   └─ DiskStore.Close()

7. Close TiKV client
   └─ client.Close()

8. Exit
```

### 6.2 Shutdown Implementation

```go
func (s *Server) Shutdown() error {
    log.Println("Shutting down KVCache instance...")
    
    // 1. Stop servers
    s.grpcServer.GracefulStop()
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    s.httpServer.Shutdown(ctx)
    
    // 2. Cancel background goroutines
    s.cancel()
    
    // 3. Wait for goroutines to finish
    s.wg.Wait()
    
    // 4. Deregister from TiKV
    key := "/kvcache/instances/" + s.config.InstanceName
    if err := s.tikvClient.Delete(key); err != nil {
        log.Printf("Failed to deregister from TiKV: %v", err)
    } else {
        log.Println("Deregistered from TiKV")
    }
    
    // 5. Close storage
    if err := s.storage.Close(); err != nil {
        log.Printf("Failed to close storage: %v", err)
    }
    
    // 6. Close TiKV client
    if err := s.tikvClient.Close(); err != nil {
        log.Printf("Failed to close TiKV client: %v", err)
    }
    
    log.Println("Shutdown complete")
    return nil
}
```

### 6.3 Crash Recovery

If the instance crashes without graceful shutdown:

- TiKV still has the instance entry
- After 5 seconds without heartbeat, SDKs mark the instance as offline
- When the instance restarts, it overwrites the TiKV entry with fresh data
- Old heartbeat timestamp is replaced, SDKs see the instance as online again

## 7. Integration with Existing Components

### 7.1 Storage Layer Integration

```go
// storage.go - Add capacity reporting
type Storage interface {
    // ... existing methods ...
    
    GetCapacity() (capacity, available, used int64, err error)
}

// rocksdb.go - Implementation
func (r *RocksDBStorage) GetCapacity() (capacity, available, used int64, err error) {
    capacity, available = getDiskCapacity(r.config.DataDir)
    used = capacity - available
    return capacity, available, used, nil
}
```

### 7.2 Configuration Integration

```go
// config.go - Add distributed config fields
type Config struct {
    // Existing fields
    RocksDB RocksDBConfig
    GRPC    GRPCConfig
    HTTP    HTTPConfig
    // ...
    
    // New distributed fields
    InstanceName string
    NodeName     string
    GRPCAddr     string
    TiKVPD       string
}
```

### 7.3 Main Entry Point

```go
func main() {
    // Parse flags
    var (
        name     = flag.String("name", "", "Instance name")
        node     = flag.String("node", "", "Node name")
        addr     = flag.String("addr", "", "gRPC address")
        tikvPD   = flag.String("tikv-pd", "", "TiKV PD address")
        dataDir  = flag.String("data-dir", "./data", "Data directory")
        valueDir = flag.String("value-dir", "./value_data", "Value directory")
    )
    flag.Parse()
    
    // Build config
    cfg := &Config{
        InstanceName: *name,
        NodeName:     *node,
        GRPCAddr:     *addr,
        TiKVPD:       *tikvPD,
        DataDir:      *dataDir,
        ValueDir:     *valueDir,
    }
    
    // Validate
    if err := validateConfig(cfg); err != nil {
        log.Fatalf("Invalid config: %v", err)
    }
    
    // Create and start server
    server, err := NewServer(cfg)
    if err != nil {
        log.Fatalf("Failed to create server: %v", err)
    }
    
    if err := server.Start(); err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }
    
    // Wait for shutdown signal
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh
    
    // Graceful shutdown
    server.Shutdown()
}
```

## 8. Multi-Instance Deployment

### 8.1 Deployment Script

```bash
#!/bin/bash
# start-instances.sh

NODES=("nodeA:192.168.1.10" "nodeB:192.168.1.11" "nodeC:192.168.1.12")
TIKV_PD="192.168.1.100:2379"
INSTANCES_PER_NODE=2

for node_spec in "${NODES[@]}"; do
    IFS=':' read -r node ip <<< "$node_spec"
    
    for i in $(seq 1 $INSTANCES_PER_NODE); do
        BASE_PORT=$((33000 + (i-1)*2))
        GRPC_PORT=$BASE_PORT
        INSTANCE_NAME="${node,,}-${i}"
        
        DATA_DIR="/data/${INSTANCE_NAME}"
        mkdir -p "$DATA_DIR"
        
        echo "Starting $INSTANCE_NAME on $node:$GRPC_PORT..."
        
        nohup ./kvcache \
            --name="$INSTANCE_NAME" \
            --node="$node" \
            --addr="${ip}:${GRPC_PORT}" \
            --tikv-pd="$TIKV_PD" \
            --data-dir="${DATA_DIR}/data" \
            --value-dir="${DATA_DIR}/value_data" \
            > "${DATA_DIR}/kvcache.log" 2>&1 &
        
        echo $! > "${DATA_DIR}/kvcache.pid"
        sleep 1
    done
done

echo "All instances started."
```

### 8.2 Verification

```bash
#!/bin/bash
# check-instances.sh

for node in nodeA nodeB nodeC; do
    echo "=== Node: $node ==="
    for pidfile in /data/${node}-*/kvcache.pid; do
        if [ -f "$pidfile" ]; then
            pid=$(cat "$pidfile")
            name=$(basename $(dirname "$pidfile"))
            if ps -p "$pid" > /dev/null; then
                echo "  ✓ $name (PID: $pid)"
            else
                echo "  ✗ $name (dead, stale PID: $pid)"
            fi
        fi
    done
done
```

## 9. Monitoring and Observability

### 9.1 TiKV Queries

```bash
# List all registered instances
tikv-client scan --prefix="/kvcache/instances/"

# Check specific instance
tikv-client get --key="/kvcache/instances/a1"

# Output:
# {
#   "name": "a1",
#   "node": "nodeA",
#   "addr": "192.168.1.10:33000",
#   "capacity": 107374182400,
#   "available": 75161927680,
#   "used": 32212254720,
#   "start_time": 1234567890
# }
```

### 9.2 Prometheus Metrics (Existing)

The server already exposes Prometheus metrics via `/metrics`:

- `cachefs_kv_ops_total` - Operation counts
- `cachefs_kv_op_errors_total` - Error counts by operation
- `cachefs_kv_op_latency_seconds` - Latency histograms
- `cachefs_storage_disk_capacity_bytes` - Disk capacity (new)
- `cachefs_storage_disk_available_bytes` - Disk available (new)

### 9.3 New Capacity Metrics

```go
func (s *Server) registerMetrics() {
    s.metrics.DiskCapacity = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "cachefs",
        Subsystem: "storage",
        Name:      "disk_capacity_bytes",
        Help:      "Total disk capacity in bytes",
    })
    
    s.metrics.DiskAvailable = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "cachefs",
        Subsystem: "storage",
        Name:      "disk_available_bytes",
        Help:      "Available disk space in bytes",
    })
    
    prometheus.MustRegister(s.metrics.DiskCapacity, s.metrics.DiskAvailable)
}

func (s *Server) updateMetrics() {
    capacity, available, _, _ := s.storage.GetCapacity()
    s.metrics.DiskCapacity.Set(float64(capacity))
    s.metrics.DiskAvailable.Set(float64(available))
}
```

## 10. Failure Scenarios

### 10.1 Instance Crash

```
Symptoms:
  - Process dies unexpectedly
  - TiKV entry remains (with old heartbeat timestamp)

Recovery:
  1. After 5s without heartbeat update, SDKs mark instance as offline
  2. When instance restarts, it overwrites TiKV entry with fresh timestamp
  3. SDKs see the instance as online again after next refresh cycle
  
Impact:
  - Keys assigned to this instance are inaccessible until restart
  - No data loss (RocksDB data persists on disk)
```

### 10.2 TiKV Unavailable

```
Symptoms:
  - Instance cannot write heartbeat
  - SDKs cannot fetch instance list

Impact:
  - Instance continues running (RocksDB is independent)
  - New SDKs cannot discover this instance
  - Existing SDKs use last known instance list (stale)
  
Recovery:
  - When TiKV recovers, heartbeat resumes
  - SDKs refresh instance list
  - No data loss
```

### 10.3 Network Partition

```
Symptoms:
  - Instance cannot reach TiKV
  - SDKs cannot reach instance

Impact:
  - Instance marked as offline by SDKs
  - Keys assigned to this instance inaccessible
  
Recovery:
  - When network recovers, heartbeat resumes
  - SDKs rediscover the instance
  
Mitigation:
  - Deploy instances across multiple network zones
  - SDK retries with backoff
```

### 10.4 Disk Full

```
Symptoms:
  - available = 0
  - RocksDB writes fail

Impact:
  - Instance cannot accept new writes
  - Reads still work
  
Recovery:
  1. Eviction manager triggers (if configured)
  2. Delete large values from disk store
  3. Compact RocksDB
  
Prevention:
  - SDK skips instances with > 80% usage
  - Monitor disk usage alerts at 70% and 90%
```

## 11. Security Considerations

### 11.1 Network Security

```
Current: Plain TCP for gRPC and TiKV communication

Future enhancements:
  - TLS encryption for gRPC (mutual TLS)
  - TLS for TiKV communication
  - Network segmentation (private subnet for TiKV)
```

### 11.2 Authentication

```
Current: No authentication

Future enhancements:
  - API key authentication for gRPC
  - Instance tokens for TiKV registration
  - Admin interface authentication
```

### 11.3 Data Security

```
Current: Data stored in plaintext on disk

Future enhancements:
  - Encryption at rest (RocksDB encryption)
  - Encryption in transit (TLS)
  - Key rotation support
```

## 12. Performance Tuning

### 12.1 Heartbeat Tuning

```
Default: 1s interval, 5s timeout

High-frequency workloads:
  - Keep 1s interval for fast failure detection
  - Reduces window of stale instance references

Conservative tuning:
  - Increase to 5s interval, 15s timeout
  - Reduces TiKV load (5x less writes)
  - Slower failure detection
```

### 12.2 RocksDB Tuning

```go
options := gorocksdb.NewDefaultOptions()

// Increase write buffer for high-throughput writes
options.SetWriteBufferSize(128 * 1024 * 1024) // 128MB
options.SetMaxWriteBufferNumber(3)

// Increase block cache for read-heavy workloads
blockCache := gorocksdb.NewLRUCache(2 * 1024 * 1024 * 1024) // 2GB
options.SetBlockCache(blockCache)

// Enable bloom filters for point lookups
filter := gorocksdb.NewBloomFilter(10)  // 10 bits per key
options.SetFilterPolicy(filter)

// Compaction settings
options.SetMaxBackgroundCompactions(4)
options.SetMaxBackgroundFlushes(2)
```

### 12.3 gRPC Tuning

```go
grpcServer := grpc.NewServer(
    grpc.MaxRecvMsgSize(16 * 1024 * 1024),  // 16MB
    grpc.MaxSendMsgSize(16 * 1024 * 1024),
    grpc.KeepaliveParams(keepalive.ServerParameters{
        MaxConnectionIdle: 5 * time.Minute,
        MaxConnectionAge: 60 * time.Minute,
        Time: 2 * time.Hour,
        Timeout: 20 * time.Second,
    }),
)
```

## 13. Future Enhancements

- [ ] **Data Replication**: Support N-way replication for fault tolerance
- [ ] **Automatic Rebalancing**: Migrate keys from overloaded instances
- [ ] **Consistent Hashing**: Replace SDK random selection with consistent hashing
- [ ] **Backup/Restore**: Automated backup to S3/HDFS
- [ ] **Read Replicas**: Support read-only replicas for read-heavy workloads
- [ ] **Compression**: Optional Snappy/Zstd compression for large values
- [ ] **Multi-tenancy**: Namespace isolation for multi-tenant deployments
