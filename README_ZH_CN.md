# KVCache - 高性能键值存储服务

## 项目简介

KVCache是一个基于Go语言开发的高性能键值存储服务，使用RocksDB作为底层存储引擎，同时提供gRPC和HTTP接口，支持大规模数据存储和快速访问。

## 功能特性

- **高性能存储**：基于RocksDB引擎，提供高效的键值存储和检索
- **双接口支持**：同时提供gRPC和HTTP RESTful接口
- **大值存储**：自动将大值存储到磁盘，优化内存使用
- **批量操作**：支持批量设置、获取和删除操作
- **数据扫描**：支持基于前缀的数据扫描
- **配置管理**：支持运行时配置更新
- **监控指标**：集成Prometheus监控
- **健康检查**：提供服务健康状态检查
- **自动淘汰**：基于磁盘使用率的自动数据淘汰机制

## 技术栈

- **语言**：Go 1.18+
- **存储引擎**：RocksDB (via gorocksdb)
- **RPC框架**：gRPC
- **HTTP框架**：Gin
- **监控**：Prometheus
- **序列化**：Protocol Buffers

## 项目结构

```
├── api/             # API层，包含gRPC和HTTP服务器实现
│   ├── grpc_server.go
│   └── http_server.go
├── config/          # 配置模块
│   ├── config.go
│   └── config_test.go
├── doc/             # 项目文档
├── proto/           # Protocol Buffers定义
│   ├── kv.pb.go
│   ├── kv.proto
│   └── kv_grpc.pb.go
├── service/         # 业务逻辑层
│   ├── kv_service.go
│   ├── metrics.go
│   └── service_test.go
├── storage/         # 存储层
│   ├── disk_store.go
│   ├── eviction.go
│   ├── rocksdb.go
│   ├── storage.go
│   └── storage_test.go
├── test/            # 测试代码
│   └── api/
├── web/             # Web前端
├── main.go          # 主入口文件
├── go.mod           # Go模块定义
└── go.sum           # 依赖版本锁定
```

## 快速开始

### 环境要求

- Go 1.18或更高版本
- GCC 4.8或更高版本（RocksDB依赖）
- Git

### 安装依赖

```bash
# 克隆项目
git clone https://github.com/yourusername/kvcache.git
cd kvcache

# 安装依赖
go mod download
```

### 编译运行

```bash
# 编译
go build -o kvcache .

# 运行
./kvcache

# 或者直接运行
go run main.go
```

服务启动后，默认监听以下端口：
- gRPC服务：50051
- HTTP服务：8080
- 监控指标：/metrics

## API接口

### HTTP接口

#### 设置键值对
- **URL**: `/api/v1/set`
- **方法**: POST
- **请求体**:
  ```json
  {
    "key": "example",
    "value": "hello world",
    "ttl": 0
  }
  ```
- **响应**:
  ```json
  {
    "success": true,
    "message": "key set successfully"
  }
  ```

#### 获取值
- **URL**: `/api/v1/get/{key}`
- **方法**: GET
- **响应**:
  ```json
  {
    "key": "example",
    "value": "hello world"
  }
  ```

#### 删除键值对
- **URL**: `/api/v1/delete/{key}`
- **方法**: DELETE
- **响应**:
  ```json
  {
    "success": true,
    "message": "key deleted successfully"
  }
  ```

#### 扫描键值对
- **URL**: `/api/v1/scan?prefix=user&limit=100`
- **方法**: GET
- **响应**:
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

#### 批量操作
- **批量设置**: `/api/v1/mset` (POST)
- **批量获取**: `/api/v1/mget` (POST)
- **批量删除**: `/api/v1/mdelete` (POST)

#### 配置管理
- **获取配置**: `/api/v1/config` (GET)
- **更新配置**: `/api/v1/config` (POST)

### gRPC接口

gRPC接口定义在 `proto/kv.proto` 文件中，包含以下方法：

- `Set` - 设置键值对
- `Get` - 获取值
- `Delete` - 删除键值对
- `ScanKeys` - 扫描键
- `ScanKeyValues` - 扫描键值对
- `MSet` - 批量设置
- `MGet` - 批量获取
- `MDelete` - 批量删除
- `GetConfig` - 获取配置
- `UpdateConfig` - 更新配置

## 配置说明

默认配置存储在 `config/config.go` 文件中，主要配置项包括：

- **RocksDB**:
  - `path`: RocksDB数据存储路径，默认 `./data`
  - `options`: RocksDB选项

- **服务端口**:
  - `grpc.port`: gRPC服务端口，默认 50051
  - `http.port`: HTTP服务端口，默认 8080

- **值存储**:
  - `value.disk_threshold`: 大值存储阈值，默认 1MB
  - `value.disk_path`: 大值存储路径，默认 `./value_data`

- **淘汰机制**:
  - `eviction.enabled`: 是否启用淘汰，默认 true
  - `eviction.disk_usage_threshold`: 磁盘使用阈值，默认 80%
  - `eviction.check_interval`: 检查间隔，默认 60秒
  - `eviction.batch_size`: 批量淘汰大小，默认 100

- **监控**:
  - `monitoring.enabled`: 是否启用监控，默认 true
  - `monitoring.metrics_path`: 指标路径，默认 `/metrics`
  - `monitoring.health_path`: 健康检查路径，默认 `/api/v1/health`

## 监控指标

服务集成了Prometheus监控，提供以下指标：

- **操作延迟**:
  - `kv_set_latency_seconds`: 设置操作延迟
  - `kv_get_latency_seconds`: 获取操作延迟
  - `kv_delete_latency_seconds`: 删除操作延迟
  - `kv_scan_latency_seconds`: 扫描操作延迟

- **操作计数**:
  - `kv_sets_total`: 设置操作总数
  - `kv_gets_total`: 获取操作总数
  - `kv_deletes_total`: 删除操作总数
  - `kv_scans_total`: 扫描操作总数

- **错误计数**:
  - `kv_set_errors_total`: 设置错误总数
  - `kv_get_errors_total`: 获取错误总数
  - `kv_delete_errors_total`: 删除错误总数
  - `kv_scan_errors_total`: 扫描错误总数

- **配置**:
  - `kv_config_updates_total`: 配置更新总数

- **健康检查**:
  - `kv_health_checks_total`: 健康检查总数
  - `kv_health_check_latency_seconds`: 健康检查延迟

## 部署建议

1. **数据目录**:
   - 确保RocksDB数据目录有足够的磁盘空间
   - 考虑使用SSD存储以获得更好的性能

2. **内存配置**:
   - 根据数据量和并发访问量调整系统内存
   - RocksDB会使用部分内存作为缓存

3. **网络配置**:
   - 根据实际需要调整gRPC和HTTP服务端口
   - 考虑使用负载均衡器分发请求

4. **监控告警**:
   - 配置Prometheus监控和Grafana仪表盘
   - 设置磁盘使用率和服务健康状态的告警

5. **备份策略**:
   - 定期备份RocksDB数据目录
   - 考虑使用RocksDB的检查点功能进行增量备份

## 常见问题

### 1. 服务启动失败

**可能原因**:
- RocksDB依赖库未正确安装
- 端口被占用
- 数据目录权限不足

**解决方案**:
- 检查GCC版本是否满足要求
- 检查端口使用情况，修改配置文件中的端口设置
- 确保数据目录有读写权限

### 2. 性能问题

**可能原因**:
- 数据量过大
- 内存不足
- 磁盘I/O瓶颈

**解决方案**:
- 增加系统内存
- 使用SSD存储
- 调整RocksDB配置参数
- 考虑分片存储

### 3. 数据丢失

**可能原因**:
- 服务异常崩溃
- 磁盘故障
- 淘汰机制误删数据

**解决方案**:
- 定期备份数据
- 使用RAID存储
- 调整淘汰机制参数

### 4. 连接超时

**可能原因**:
- 网络延迟
- 服务负载过高
- 批量操作数据量过大

**解决方案**:
- 优化网络环境
- 增加服务实例
- 减少单次批量操作的数据量

## 许可证

MIT License

## 贡献

欢迎提交Issue和Pull Request！

## 联系方式

如有问题，请联系项目维护者。