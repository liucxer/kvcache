# KVCache - High Performance Key-Value Storage Service

## Project Introduction

KVCache is a high-performance key-value storage service developed in Go language, using RocksDB as the underlying storage engine, and providing both gRPC and HTTP interfaces, supporting large-scale data storage and fast access.

## Features

- **High Performance Storage**: Based on RocksDB engine, providing efficient key-value storage and retrieval
- **Dual Interface Support**: Providing both gRPC and HTTP RESTful interfaces
- **Large Value Storage**: Automatically storing large values to disk, optimizing memory usage
- **Batch Operations**: Supporting batch set, get and delete operations
- **Data Scanning**: Supporting prefix-based data scanning
- **Configuration Management**: Supporting runtime configuration updates
- **Monitoring Metrics**: Integrated with Prometheus monitoring
- **Health Check**: Providing service health status check
- **Automatic Eviction**: Automatic data eviction mechanism based on disk usage

## Technology Stack

- **Language**: Go 1.18+
- **Storage Engine**: RocksDB (via gorocksdb)
- **RPC Framework**: gRPC
- **HTTP Framework**: Gin
- **Monitoring**: Prometheus
- **Serialization**: Protocol Buffers

## Project Structure

```
├── api/             # API layer, containing gRPC and HTTP server implementations
│   ├── grpc_server.go
│   └── http_server.go
├── config/          # Configuration module
│   ├── config.go
│   └── config_test.go
├── proto/           # Protocol Buffers definitions
│   ├── kv.pb.go
│   ├── kv.proto
│   └── kv_grpc.pb.go
├── service/         # Business logic layer
│   ├── kv_service.go
│   ├── metrics.go
│   ├── performance_test.go  # Performance test cases
│   └── service_test.go
├── storage/         # Storage layer
│   ├── disk_store.go
│   ├── eviction.go
│   ├── rocksdb.go
│   ├── storage.go
│   └── storage_test.go
├── test/            # Test code
│   └── api/
│       ├── grpc_performance_test.go  # gRPC performance test cases
│       ├── grpc_test.go
│       └── http_test.go
├── web/             # Web frontend
├── main.go          # Main entry file
├── Makefile         # Build and test scripts
├── go.mod           # Go module definition
└── go.sum           # Dependency version lock
```

## Quick Start

### Environment Requirements

- Go 1.18 or higher
- GCC 4.8 or higher (RocksDB dependency)
- Git

### Install Dependencies

```bash
# Clone the project
git clone https://github.com/yourusername/kvcache.git
cd kvcache

# Install dependencies
go mod download
```

### Build and Run

#### Using Makefile (Recommended)

```bash
# Build
make build

# Run
make run

# Clean
make clean
```

#### Using Go Commands

```bash
# Build with CGO environment variables
CGO_CFLAGS="-I/opt/homebrew/include" \
CGO_LDFLAGS="-L/opt/homebrew/lib  -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd" \
  go build -o kvcache .

# Run
./kvcache

# Or run directly with CGO environment variables
CGO_CFLAGS="-I/opt/homebrew/include" \
CGO_LDFLAGS="-L/opt/homebrew/lib  -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd" \
  go run main.go
```

After starting the service, it listens on the following ports by default:
- gRPC service: 50051
- HTTP service: 8080
- Monitoring metrics: /metrics

## API Interface

### HTTP Interface

#### Set Key-Value Pair
- **URL**: `/api/v1/set`
- **Method**: POST
- **Request Body**:
  ```json
  {
    "key": "example",
    "value": "hello world",
    "ttl": 0
  }
  ```
- **Response**:
  ```json
  {
    "success": true,
    "message": "key set successfully"
  }
  ```

#### Get Value
- **URL**: `/api/v1/get/{key}`
- **Method**: GET
- **Response**:
  ```json
  {
    "key": "example",
    "value": "hello world"
  }
  ```

#### Delete Key-Value Pair
- **URL**: `/api/v1/delete/{key}`
- **Method**: DELETE
- **Response**:
  ```json
  {
    "success": true,
    "message": "key deleted successfully"
  }
  ```

#### Scan Key-Value Pairs
- **URL**: `/api/v1/scan?prefix=user&limit=100`
- **Method**: GET
- **Response**:
  ```json
  {
    "prefix": "user",
    "limit": 100,
    "count": 5,
    "results": {
      "user1": "value1",
      "user2": "value2"
    }
  }
  ```

#### Batch Operations
- **Batch Set**: `/api/v1/mset` (POST)
- **Batch Get**: `/api/v1/mget` (POST)
- **Batch Delete**: `/api/v1/mdelete` (POST)

#### Configuration Management
- **Get Configuration**: `/api/v1/config` (GET)
- **Update Configuration**: `/api/v1/config` (POST)

### gRPC Interface

The gRPC interface is defined in the `proto/kv.proto` file, including the following methods:

- `Set` - Set key-value pair
- `Get` - Get value
- `Delete` - Delete key-value pair
- `ScanKeys` - Scan keys
- `ScanKeyValues` - Scan key-value pairs
- `MSet` - Batch set
- `MGet` - Batch get
- `MDelete` - Batch delete
- `GetConfig` - Get configuration
- `UpdateConfig` - Update configuration

## Testing

### Running Tests

The project includes comprehensive unit tests and integration tests. You can run the tests using the following commands:

#### Using Makefile

```bash
# Run all tests
make test

# Run config tests
make test-config

# Run service tests
make test-service

# Run storage tests
make test-storage

# Run API tests
make test-api

# Run HTTP API tests
make test-http

# Run gRPC API tests
make test-grpc

# Run tests with verbose output
make test-verbose
```

#### Using Go Commands

```bash
# Run all tests
CGO_CFLAGS="-I/opt/homebrew/include" \
CGO_LDFLAGS="-L/opt/homebrew/lib  -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd" \
  go test ./...

# Run specific package tests
CGO_CFLAGS="-I/opt/homebrew/include" \
CGO_LDFLAGS="-L/opt/homebrew/lib  -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd" \
  go test ./config
```

## Performance Testing

### Running Performance Tests

The project includes performance test cases for both the service layer and gRPC client. You can run the performance tests using the following commands:

#### Using Makefile

```bash
# Run all performance tests
make test-performance

# Run set benchmark
make test-benchmark-set

# Run get benchmark
make test-benchmark-get

# Run concurrent benchmarks
make test-benchmark-concurrent

# Run mixed operations benchmark
make test-benchmark-mixed

# Run gRPC performance tests
make test-grpc-performance

# Run gRPC set benchmark
make test-grpc-benchmark-set

# Run gRPC get benchmark
make test-grpc-benchmark-get

# Run gRPC concurrent benchmarks
make test-grpc-benchmark-concurrent

# Run gRPC mixed operations benchmark
make test-grpc-benchmark-mixed
```

### Performance Test Results

#### Service Layer Performance

| Test Name | Operations/Second | Average Latency/Operation |
|-----------|-------------------|---------------------------|
| Set (Single-thread) | ~316,732 | ~3,762 ns |
| Get (Single-thread) | ~1,629,542 | ~783 ns |
| Set (Concurrent) | ~214,846 | ~6,049 ns |
| Get (Concurrent) | ~4,086,961 | ~309.7 ns |
| Mixed Operations | ~747,752 | ~1,459 ns |

#### gRPC Client Performance

| Test Name | Operations/Second | Average Latency/Operation |
|-----------|-------------------|---------------------------|
| Set (Single-thread) | ~20,506 | ~58,621 ns |
| Get (Single-thread) | ~21,990 | ~49,830 ns |
| Set (Concurrent) | ~65,038 | ~21,667 ns |
| Get (Concurrent) | ~95,284 | ~15,663 ns |
| Mixed Operations | ~76,860 | ~15,564 ns |

### Performance Analysis

- **Service Layer Performance**: The service layer shows excellent performance, with get operations being significantly faster than set operations. This is expected since get operations are typically faster than write operations in key-value stores.

- **gRPC Performance**: The gRPC client performance is slower than direct service calls, as expected, due to the additional overhead of network transmission, serialization, and deserialization. However, the performance is still good, especially in concurrent scenarios.

- **Concurrent Performance**: Both the service layer and gRPC client show significant performance improvements in concurrent scenarios, demonstrating the system's ability to handle multiple concurrent requests efficiently.

## Configuration

The default configuration is stored in the `config/config.go` file, and the main configuration items include:

- **RocksDB**:
  - `path`: RocksDB data storage path, default `./data`
  - `options`: RocksDB options

- **Service Ports**:
  - `grpc.port`: gRPC service port, default 50051
  - `http.port`: HTTP service port, default 8080

- **Value Storage**:
  - `value.disk_threshold`: Large value storage threshold, default 1MB
  - `value.disk_path`: Large value storage path, default `./value_data`

- **Eviction Mechanism**:
  - `eviction.enabled`: Whether to enable eviction, default true
  - `eviction.disk_usage_threshold`: Disk usage threshold, default 80%
  - `eviction.check_interval`: Check interval, default 60 seconds
  - `eviction.batch_size`: Batch eviction size, default 100

- **Monitoring**:
  - `monitoring.enabled`: Whether to enable monitoring, default true
  - `monitoring.metrics_path`: Metrics path, default `/metrics`
  - `monitoring.health_path`: Health check path, default `/api/v1/health`

## Monitoring

The service integrates with Prometheus monitoring, providing the following metrics:

- **Operation Latency**:
  - `kv_set_latency_seconds`: Set operation latency
  - `kv_get_latency_seconds`: Get operation latency
  - `kv_delete_latency_seconds`: Delete operation latency
  - `kv_scan_latency_seconds`: Scan operation latency

- **Operation Count**:
  - `kv_sets_total`: Total set operations
  - `kv_gets_total`: Total get operations
  - `kv_deletes_total`: Total delete operations
  - `kv_scans_total`: Total scan operations

- **Error Count**:
  - `kv_set_errors_total`: Total set errors
  - `kv_get_errors_total`: Total get errors
  - `kv_delete_errors_total`: Total delete errors
  - `kv_scan_errors_total`: Total scan errors

- **Configuration**:
  - `kv_config_updates_total`: Total configuration updates

- **Health Check**:
  - `kv_health_checks_total`: Total health checks
  - `kv_health_check_latency_seconds`: Health check latency

## Deployment

1. **Data Directory**:
   - Ensure the RocksDB data directory has sufficient disk space
   - Consider using SSD storage for better performance

2. **Memory Configuration**:
   - Adjust system memory based on data volume and concurrent access
   - RocksDB will use part of the memory as cache

3. **Network Configuration**:
   - Adjust gRPC and HTTP service ports according to actual needs
   - Consider using a load balancer to distribute requests

4. **Monitoring and Alerting**:
   - Configure Prometheus monitoring and Grafana dashboards
   - Set up alerts for disk usage and service health status

5. **Backup Strategy**:
   - Regularly back up the RocksDB data directory
   - Consider using RocksDB's checkpoint feature for incremental backups

## FAQ

### 1. Service Startup Failure

**Possible Causes**:
- RocksDB dependency library not installed correctly
- Port occupied
- Insufficient data directory permissions

**Solutions**:
- Check if GCC version meets requirements
- Check port usage and modify port settings in the configuration file
- Ensure the data directory has read and write permissions

### 2. Performance Issues

**Possible Causes**:
- Excessive data volume
- Insufficient memory
- Disk I/O bottleneck

**Solutions**:
- Increase system memory
- Use SSD storage
- Adjust RocksDB configuration parameters
- Consider sharded storage

### 3. Data Loss

**Possible Causes**:
- Service abnormal crash
- Disk failure
- Eviction mechanism mistakenly deleting data

**Solutions**:
- Regularly back up data
- Use RAID storage
- Adjust eviction mechanism parameters

### 4. Connection Timeout

**Possible Causes**:
- Network latency
- High service load
- Excessive batch operation data volume

**Solutions**:
- Optimize network environment
- Increase service instances
- Reduce the data volume of single batch operations

## License

MIT License

## Contribution

Welcome to submit Issues and Pull Requests!

## Contact

If you have any questions, please contact the project maintainer.