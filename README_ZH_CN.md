# KVCache - 高性能键值存储服务

## 项目简介

KVCache 是一个基于 Go 语言开发的高性能键值存储服务，使用 RocksDB 作为底层存储引擎，同时提供 gRPC 和 HTTP 接口，支持大规模数据存储和快速访问。

## 功能特性

- **高性能存储**：基于 RocksDB 引擎，提供高效的键值存储和检索
- **双接口支持**：同时提供 gRPC 和 HTTP RESTful 接口
- **大值存储**：自动将大值（>1MB）存储到独立磁盘文件，RocksDB 仅保留路径引用
- **内存缓存**：`sync.Map` 实现的一级读缓存，小值（<10KB）自动缓存与回填
- **批量操作**：支持基于 WriteBatch 的原子批量设置、获取和删除
- **数据扫描**：支持基于前缀的有序扫描
- **自动淘汰**：基于磁盘使用率的 FIFO 淘汰机制
- **运行时配置**：通过 API 动态修改服务配置
- **Prometheus 监控**：全链路操作计数、错误率、延迟直方图
- **多实例部署**：端口自动发现、进程管理脚本
- **客户端 SDK**：多后端轮询负载均衡与失败重试
- **Web 管理界面**：图形化 KV CRUD、扫描和配置管理

## 技术栈

| 组件 | 技术选型 |
|------|---------|
| 语言 | Go 1.18+ |
| 存储引擎 | RocksDB（via grocksdb CGO 绑定） |
| RPC 框架 | gRPC + Protocol Buffers |
| HTTP 框架 | Gin |
| 监控 | Prometheus client_golang |

## 架构总览

```
main.go（端口自动发现 33000-33100）
  ├── config/          → 配置结构体 + 默认值（纯数据，无 I/O）
  ├── storage/         → RocksDB 封装 + 磁盘大值存储 + 淘汰引擎
  │     └── Storage 接口
  ├── service/         → 业务逻辑 + Prometheus 指标 + 内存缓存
  │     └── 依赖 storage.Storage
  ├── api/             → gRPC Server + HTTP Server（薄适配层）
  │     └── 依赖 service.KVService
  ├── client/          → gRPC 客户端 SDK（轮询 + 故障重试）
  │     └── 依赖 proto
  └── web/             → 静态前端（Tailwind CSS 单页应用）
```

调用链单向依赖：`api → service → storage`，`main.go` 负责组装。

## 项目结构

```
├── api/                          # API 层
│   ├── grpc_server.go            #   gRPC 适配器（实现 proto 接口）
│   └── http_server.go            #   HTTP 适配器（Gin 路由）
├── client/                       # 客户端 SDK
│   ├── client.go                 #   多后端轮询 + 重试逻辑
│   └── example.go                #   使用示例
├── config/                       # 配置模块
│   ├── config.go                 #   配置结构体 + DefaultConfig()
│   └── config_test.go            #   配置测试
├── proto/                        # Protocol Buffers
│   ├── kv.proto                  #   服务定义源文件
│   ├── kv.pb.go                  #   生成的消息代码
│   └── kv_grpc.pb.go            #   生成的 gRPC stub
├── service/                      # 业务逻辑层
│   ├── kv_service.go             #   KVService（缓存 + 指标 + 转发）
│   ├── metrics.go                #   Prometheus 指标定义
│   ├── service_test.go           #   服务层测试
│   └── performance_test.go       #   基准测试
├── storage/                      # 存储层
│   ├── storage.go                #   Storage 接口定义
│   ├── rocksdb.go                #   RocksDB 实现（核心）
│   ├── disk_store.go             #   大值磁盘存储
│   ├── eviction.go               #   淘汰管理器
│   └── storage_test.go           #   存储层测试
├── test/api/                     # 集成测试
│   ├── http_test.go              #   HTTP API 测试 + 全局初始化
│   ├── grpc_test.go              #   gRPC API 测试
│   └── grpc_performance_test.go  #   gRPC 基准测试
├── web/                          # Web 前端
│   ├── index.html                #   单页管理界面
│   └── script.js                 #   前端交互逻辑
├── main.go                       # 主入口
├── start-instances.sh            # 启动多实例
├── stop-instances.sh             # 停止多实例
├── status-instances.sh           # 查看实例状态
├── Makefile                      # 构建和测试脚本
├── go.mod                        # Go 模块定义
└── go.sum                        # 依赖版本锁定
```

## 快速开始

### 环境要求

- Go 1.18 或更高版本
- GCC 4.8 或更高版本（RocksDB CGO 依赖）
- RocksDB 开发库（`librocksdb-dev` 或等价包）

### 安装依赖

```bash
git clone https://github.com/yourusername/kvcache.git
cd kvcache
go mod download
```

### 编译运行

#### 使用 Makefile（推荐）

```bash
make build    # 编译
make run      # 编译并运行
make clean    # 清理构建产物
```

#### 使用 Go 命令

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

### 端口分配

服务启动后通过**自动端口发现**机制分配端口对：
- 扫描 `33000-33100` 范围
- **偶数端口**分配给 gRPC，**奇数端口**分配给 HTTP（如 33000/33001、33002/33003）
- 分配的端口号会在启动日志中输出

## 功能详解

### 1. 键值对基本操作（Set / Get / Delete）

**写入流程（Set）**：

1. 客户端通过 HTTP `POST /api/v1/set` 传入 `{key, value, ttl}` 或通过 gRPC `Set` RPC 传入 `{key: bytes, value: bytes}`
2. `api` 层校验 key 非空，转发给 `service.KVService.Set()`
3. `KVService` 调用 `storage.Set()` 写入 RocksDB，同时记录延迟和计数指标
4. `RocksDBStorage.Set()` 根据 value 大小分流：
   - **小值**（≤ `DiskThreshold`，默认 1MB）：直接 `PutCF()` 写入 RocksDB
   - **大值**（> 阈值）：写入 `value_data/` 目录独立文件，RocksDB 存路径引用 `"__rocksdb_disk_store__://filename"`
5. 回到 `KVService`，若缓存开启且 value < `SizeThreshold`（10KB），写入内存缓存

**读取流程（Get）**：

1. HTTP `GET /api/v1/get/:key` 或 gRPC `Get` RPC
2. `KVService.Get()` 先查 `sync.Map` 缓存：
   - **缓存命中**：直接返回
   - **缓存未命中**：调用 `storage.Get()` 查 RocksDB
3. `RocksDBStorage.Get()` 判断值类型：
   - 值为空 → `found=false`
   - 值为 `"__evicted__"` → 返回"已被淘汰"错误
   - 值以 `__rocksdb_disk_store__://` 开头 → 从 `DiskStore` 读回完整数据
   - 普通值 → 复制后返回（避免 RocksDB C 层 buffer 释放问题）
4. 读回后，若 value < 缓存阈值，**回填到缓存**（缓存穿透后回填策略）

**删除流程（Delete）**：

1. HTTP `DELETE /api/v1/delete/:key` 或 gRPC `Delete` RPC
2. `storage.Delete()` 先读 RocksDB 判断类型：大值磁盘文件先删文件，再从 RocksDB 删 key
3. `KVService.Delete()` 同步从内存缓存移除 key，Key 计数 gauge 减 1

### 2. 批量操作（MSet / MGet / MDelete）

**MSet** — 原子批量写入：
- HTTP `POST /api/v1/mset` 传入 `{kvs: {k1: v1, ...}, ttl}`
- `storage.MSet()` 底层使用 RocksDB **WriteBatch**，所有 put 打包成一个原子操作提交
- 大值同样走磁盘文件，小值批量写入缓存

**MGet** — 批量读取 + 缓存穿透优化：
- `KVService.MGet()` 先遍历所有 key 查 `sync.Map`，命中的直接放入结果集
- 未命中的 key 收集起来一次性调用 `storage.MGet()`
- 读回后小于阈值的 value 回填缓存

**MDelete** — 批量删除：
- 底层使用 WriteBatch 原子删除（每条 key 先检查磁盘文件引用）
- 从内存缓存中逐条清除

### 3. 前缀扫描（Scan）

基于 RocksDB 有序迭代器实现的键前缀查询。

- gRPC 提供 `ScanKeys`（只返回 key 列表）和 `ScanKeyValues`（返回 key→value 映射）
- HTTP `GET /api/v1/scan?prefix=xxx&limit=100` 返回 key-value map
- `ScanWithValues()` 使用 `Seek(prefix)` 定位到前缀起始位置，遇到不匹配 key 时 `break`（利用有序性，效率高）
- `Scan()` 使用 `SeekToFirst()` 全量遍历过滤（效率较低）
- 两者都会跳过系统保留 key `global.config`

### 4. 大值磁盘存储（DiskStore）

为超过 1MB 的 value 设计的二级存储：

| 操作 | 实现 |
|------|------|
| Store | 计算 SHA256(data) 作为文件名写入 `value_data/`，content-addressable（相同内容自动去重） |
| Load | 根据文件名 `os.ReadFile` 读回 |
| Delete | `os.Remove` 删除文件，文件不存在时静默忽略 |

引用关系：RocksDB 中存储 `"__rocksdb_disk_store__://filename"`，读取时自动识别前缀并去磁盘加载。

### 5. 内存缓存

`KVService` 中的 `sync.Map` 一级读缓存：

- **开启条件**：`config.cache.enabled = true`（默认开启）
- **缓存阈值**：`config.cache.size_threshold = 10240`（10KB）
- **写入时机**：Set 成功后写缓存；Get/MGet 缓存未命中后回填
- **失效时机**：Delete/MDelete 时从缓存移除
- **并发安全**：`sync.Map` 原生支持并发读写

### 6. 自动淘汰机制（Eviction）

设计目标：磁盘使用率超阈值时，按创建时间从旧到新批量淘汰大值数据。

| 配置项 | 默认值 |
|--------|--------|
| `eviction.enabled` | true |
| `eviction.disk_usage_threshold` | 80% |
| `eviction.check_interval` | 60 秒 |
| `eviction.batch_size` | 每轮最多淘汰 100 条 |

**淘汰流程**：
1. `EvictionManager` 定时运行 `checkAndEvict()`
2. 超过阈值时，遍历创建时间索引，对每条大值 key 执行：
   - 删除磁盘文件
   - RocksDB 中将其值替换为 `"__evicted__"` 标记
   - 从创建时间索引中移除
3. 客户端 Get 已被淘汰的 key 会收到 `"value has been evicted"` 错误

### 7. 运行时配置管理

配置通过 `DefaultConfig()` 硬编码默认值，启动后可通过 API 动态修改。

| 配置块 | 字段 | 默认值 |
|--------|------|--------|
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

**更新方式**：
- HTTP `POST /api/v1/config`：扁平化 JSON 字段（如 `rocksdb_path`、`max_disk_usage`）
- gRPC `UpdateConfig`：嵌套 JSON
- Web 前端配置管理页

淘汰配置变更时自动重启 `EvictionManager`。

### 8. Prometheus 监控

所有指标以 `cachefs` 为 namespace，通过 `/metrics` 端点暴露。

**操作计数（Counter）**：
```
cachefs_kv_sets_total / gets_total / deletes_total / scans_total
cachefs_kv_msets_total / mgets_total / mdeletes_total
cachefs_config_updates_total
cachefs_health_checks_total
```

**错误计数（CounterVec，按 error label 分类）**：
```
cachefs_kv_set_errors_total{error="empty_key"}
cachefs_kv_get_errors_total{error="not_found"}
... 每种操作都有独立的错误计数器
```

**延迟直方图（HistogramVec，秒）**：
```
cachefs_kv_set_latency_seconds{type="kv"}
cachefs_kv_get_latency_seconds{type="config"}
... 桶范围 ExponentialBuckets(0.001, 2, 10) 即 1ms ~ 512ms
```

**状态指标（Gauge）**：
```
cachefs_kv_keys_current        # 当前 key 总数
cachefs_storage_disk_usage_bytes
cachefs_storage_memory_usage_bytes
```

## API 接口

### HTTP 接口（Gin 框架）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/set` | 设置键值对 `{key, value, ttl}` |
| GET | `/api/v1/get/:key` | 获取值 |
| DELETE | `/api/v1/delete/:key` | 删除键值对 |
| GET | `/api/v1/scan?prefix=&limit=` | 前缀扫描 |
| POST | `/api/v1/mset` | 批量设置 `{kvs: {...}, ttl}` |
| POST | `/api/v1/mget` | 批量获取 `{keys: [...]}` |
| POST | `/api/v1/mdelete` | 批量删除 `{keys: [...]}` |
| GET | `/api/v1/config` | 获取当前配置 |
| POST | `/api/v1/config` | 更新配置 |
| GET | `/health` | 健康检查 |
| GET | `/metrics` | Prometheus 指标 |
| GET | `/` | 重定向到 `/web/index.html` |
| GET | `/web/*` | Web 管理前端静态文件 |

#### 请求/响应示例

**Set**：
```json
// POST /api/v1/set
{"key": "example", "value": "hello world", "ttl": 0}
// → {"success": true, "message": "key set successfully"}
```

**Get**：
```json
// GET /api/v1/get/example
// → {"key": "example", "value": "hello world"}
```

**Scan**：
```json
// GET /api/v1/scan?prefix=user&limit=100
// → {"prefix": "user", "limit": 100, "count": 2, "results": {"user1": "v1", "user2": "v2"}}
```

### gRPC 接口

定义在 `proto/kv.proto` 中，共两个 service：

**KeyValueService**（10 个 RPC）：

| RPC | 说明 | 请求类型 | 响应类型 |
|-----|------|---------|---------|
| `Set` | 设置键值对 | `SetRequest{key, value}` | `SetResponse{success, error}` |
| `Get` | 获取值 | `GetRequest{key}` | `GetResponse{value, found, error}` |
| `Delete` | 删除键值对 | `DeleteRequest{key}` | `DeleteResponse{success, error}` |
| `ScanKeys` | 扫描键 | `ScanRequest{prefix}` | `ScanKeysResponse{keys}` |
| `ScanKeyValues` | 扫描键值对 | `ScanRequest{prefix}` | `ScanKeyValuesResponse{key_values}` |
| `MSet` | 批量设置 | `MSetRequest{key_values}` | `MSetResponse{success, error}` |
| `MGet` | 批量获取 | `MGetRequest{keys}` | `MGetResponse{key_values}` |
| `MDelete` | 批量删除 | `MDeleteRequest{keys}` | `MDeleteResponse{success, error}` |
| `GetConfig` | 获取配置 | `GetConfigRequest{}` | `GetConfigResponse{config}` |
| `UpdateConfig` | 更新配置 | `UpdateConfigRequest{config}` | `UpdateConfigResponse{success, error}` |

**Health**（1 个 RPC）：

| RPC | 说明 |
|-----|------|
| `Check` | 健康检查，返回 UNKNOWN / SERVING / NOT_SERVING / SERVICE_UNKNOWN |

proto 中 key 和 value 类型统一为 `bytes`。

**HTTP vs gRPC 差异**：
- HTTP Set 接受 TTL（秒），gRPC proto 中无 TTL 字段（硬编码为 0）
- HTTP Scan 返回 key-value map，gRPC 可分别获取 key 列表或完整映射

## 客户端 SDK

`client/` 包提供 gRPC 客户端，支持多后端负载均衡：

```go
serverAddrs := []string{
    "localhost:33000", "localhost:33002", "localhost:33004",
}
client, _ := client.NewClient(serverAddrs)

// 轮询负载均衡 + 失败自动重试
client.Set(ctx, "key", []byte("value"), 0)
value, _ := client.Get(ctx, "key")
client.Delete(ctx, "key")
```

- **负载均衡**：round-robin 轮询，每次请求分发到不同后端
- **故障重试**：某个后端失败时，自动尝试下一个后端

## Web 管理前端

`web/` 目录提供单页管理界面（Tailwind CSS + 原生 JS）：

| 标签页 | 功能 |
|--------|------|
| 概览 | 健康检查、监控指标 |
| 单键操作 | Set（支持 TTL）、Get、Delete 表单 |
| 批量操作 | MSet / MGet / MDelete，动态增删键值行 |
| 扫描 | 按前缀搜索，支持只看 key 或 key+value |
| 配置管理 | 展示当前 JSON 配置，表单更新 |

访问根路径 `/` 自动跳转到 `/web/index.html`。

## 多实例部署

### 端口分配

`main.go` 在启动时扫描 `33000-33100` 范围内的可用端口对（偶数=gRPC，奇数=HTTP），每个实例自动分配不冲突的端口。

### 实例管理脚本

```bash
# 启动实例（每个路径一个独立实例）
./start-instances.sh /data/inst1 /data/inst2 /data/inst3

# 查看实例状态（端口、进程、日志位置）
./status-instances.sh

# 停止实例
./stop-instances.sh /data/inst1 /data/inst2
./stop-instances.sh all    # 停止所有实例
```

每个实例在各自的目录中维护独立的：
- `data/` — RocksDB 数据
- `value_data/` — 大值磁盘存储
- `{name}.log` — 运行日志
- `{name}.pid` — 进程 PID 文件

## 测试

### 运行测试

```bash
make test                 # 全部单元测试 + 集成测试
make test-config          # config 包
make test-service         # service 包
make test-storage         # storage 包
make test-api             # 集成测试（HTTP + gRPC）
make test-http            # 仅 HTTP API 测试
make test-grpc            # 仅 gRPC API 测试
make test-verbose         # 详细输出
```

### 基准测试

```bash
make test-performance             # 服务层全部基准
make test-benchmark-set           # Set 单线程基准
make test-benchmark-get           # Get 单线程基准
make test-benchmark-concurrent    # 并发基准
make test-benchmark-mixed         # 混合操作（80% 读 + 20% 写）
make test-grpc-performance        # gRPC 层全部基准
make test-grpc-benchmark-set/get/concurrent/mixed
```

### 测试覆盖率

```bash
make test-coverage          # 全部测试覆盖率
make test-coverage-html     # 生成 HTML 覆盖率报告
make test-coverage-storage  # storage 包覆盖率
```

### 性能测试结果

#### 服务层

| 测试名称 | 操作/秒 | 平均延迟/操作 |
|---------|---------|--------------|
| Set（单线程） | ~316,732 | ~3,762 ns |
| Get（单线程） | ~1,629,542 | ~783 ns |
| Set（并发） | ~214,846 | ~6,049 ns |
| Get（并发） | ~4,086,961 | ~309.7 ns |
| 混合操作 | ~747,752 | ~1,459 ns |

#### gRPC 客户端

| 测试名称 | 操作/秒 | 平均延迟/操作 |
|---------|---------|--------------|
| Set（单线程） | ~20,506 | ~58,621 ns |
| Get（单线程） | ~21,990 | ~49,830 ns |
| Set（并发） | ~65,038 | ~21,667 ns |
| Get（并发） | ~95,284 | ~15,663 ns |
| 混合操作 | ~76,860 | ~15,564 ns |

## 部署建议

1. **数据目录**：确保 RocksDB 数据目录有足够磁盘空间，推荐使用 SSD
2. **块设备 Readahead**：服务启动时会自动将 `--value-dir` 所在块设备的 `read_ahead_kb` 调大（默认目标 4096 KB；启动参数 `--readahead-kb`，设为 `0` 可关闭）。大 value 顺序读对该项敏感，内核默认 128 KB 在 NVMe 上会损失约 25% 读带宽。需要对 `/sys/block/*/queue/read_ahead_kb` 的写权限（root 运行或提前手动设置）；设置失败只会记录告警，不影响启动。
3. **内存配置**：根据数据量调整系统内存，RocksDB 会使用部分内存作为 BlockCache（默认 64MB）
3. **网络配置**：根据需要使用负载均衡器分发请求到多实例
4. **监控告警**：配置 Prometheus 抓取 `/metrics`，配合 Grafana 仪表盘；设置磁盘使用率告警
5. **备份策略**：定期备份 RocksDB 数据目录，或使用 RocksDB Checkpoint 增量备份

## 常见问题

### 服务启动失败
**可能原因**：RocksDB 依赖未安装、端口被占用、数据目录权限不足
**排查**：检查 GCC 和 librocksdb 版本，确认 33000-33100 端口未被占用

### 性能问题
**可能原因**：数据量过大、内存不足、磁盘 I/O 瓶颈
**排查**：使用 `make test-coverage` 定位瓶颈；调整 RocksDB BlockCache 大小；考虑多实例分片

### 连接超时
**可能原因**：网络延迟、服务负载过高、单次批量操作数据量过大
**排查**：增加服务实例；减少单次批量操作的 key 数量

## 许可证

MIT License
