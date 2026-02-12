# 后端服务需求文档

## 1. 项目概述

本项目旨在实现一个基于 Golang 的高性能键值存储后端服务，对外提供 gRPC 和 HTTP 两种接口方式，底层使用 RocksDB 作为存储引擎，提供基本的键值操作功能。

## 2. 功能需求

### 2.1 核心功能

| 功能 | 描述 | 接口类型 |
|------|------|----------|
| Set | 设置键值对 (key = value) | gRPC/HTTP |
| Get | 根据键获取值 | gRPC/HTTP |
| Delete | 根据键删除数据 | gRPC/HTTP |
| Scan (Key Prefix) | 根据键前缀扫描，返回匹配的键列表 | gRPC/HTTP |
| Scan (Key Prefix with Value) | 根据键前缀扫描，返回匹配的键值对 | gRPC/HTTP |
| MSet | 批量设置键值对 | gRPC/HTTP |
| MGet | 批量获取值 | gRPC/HTTP |
| MDelete | 批量删除键值对 | gRPC/HTTP |
| GetConfig | 获取服务配置 | gRPC/HTTP |
| UpdateConfig | 更新服务配置 | gRPC/HTTP |

### 2.2 技术需求

1. **存储引擎**：使用 RocksDB 作为底层存储引擎
2. **接口类型**：同时提供 gRPC 和 HTTP 接口
3. **编程语言**：使用 Golang 实现
4. **性能要求**：高性能、低延迟
5. **可靠性**：数据持久化存储
6. **大值存储**：自动将大值存储到磁盘，优化内存使用
7. **监控**：集成 Prometheus 监控
8. **健康检查**：提供服务健康状态检查
9. **自动淘汰**：基于磁盘使用率的自动数据淘汰机制

## 3. 技术栈

| 技术 | 版本 | 用途 |
|------|------|------|
| Golang | 1.20+ | 主要开发语言 |
| RocksDB | 7.0+ | 底层存储引擎 |
| gRPC | 1.50+ | 高性能 RPC 接口 |
| Protocol Buffers | 3.0+ | 数据序列化 |
| Gin | 1.9+ | HTTP 接口框架 |
| Prometheus | 2.0+ | 监控系统集成 |

## 4. 架构设计

### 4.1 系统架构

```
┌───────────────┐     ┌───────────────┐     ┌───────────────┐
│   gRPC 接口   │────▶│  业务逻辑层   │────▶│  RocksDB 存储  │
└───────────────┘     │               │     └───────────────┘
┌───────────────┐     │               │
│  HTTP 接口    │────▶│               │
└───────────────┘     └───────────────┘
```

### 4.2 模块划分

1. **api**：接口层，包含 gRPC 和 HTTP 接口定义
2. **service**：业务逻辑层，处理核心业务逻辑
3. **storage**：存储层，封装 RocksDB 操作
4. **proto**：Protocol Buffers 定义文件
5. **config**：配置管理
6. **utils**：工具函数

## 5. 接口定义

### 5.1 gRPC 接口

#### 5.1.1 服务定义

```protobuf
service KeyValueService {
  rpc Set(SetRequest) returns (SetResponse);
  rpc Get(GetRequest) returns (GetResponse);
  rpc Delete(DeleteRequest) returns (DeleteResponse);
  rpc ScanKeys(ScanRequest) returns (ScanKeysResponse);
  rpc ScanKeyValues(ScanRequest) returns (ScanKeyValuesResponse);
  rpc MSet(MSetRequest) returns (MSetResponse);
  rpc MGet(MGetRequest) returns (MGetResponse);
  rpc MDelete(MDeleteRequest) returns (MDeleteResponse);
  rpc GetConfig(GetConfigRequest) returns (GetConfigResponse);
  rpc UpdateConfig(UpdateConfigRequest) returns (UpdateConfigResponse);
}
```

#### 5.1.2 消息定义

```protobuf
message SetRequest {
  string key = 1;
  string value = 2;
}

message SetResponse {
  bool success = 1;
  string error = 2;
}

message GetRequest {
  string key = 1;
}

message GetResponse {
  string value = 1;
  bool found = 2;
  string error = 3;
}

message DeleteRequest {
  string key = 1;
}

message DeleteResponse {
  bool success = 1;
  string error = 2;
}

message ScanRequest {
  string prefix = 1;
}

message ScanKeysResponse {
  repeated string keys = 1;
  string error = 2;
}

message ScanKeyValuesResponse {
  map<string, string> key_values = 1;
  string error = 2;
}

message MSetRequest {
  map<string, string> key_values = 1;
}

message MSetResponse {
  bool success = 1;
  string error = 2;
}

message MGetRequest {
  repeated string keys = 1;
}

message MGetResponse {
  map<string, string> key_values = 1;
  string error = 2;
}

message MDeleteRequest {
  repeated string keys = 1;
}

message MDeleteResponse {
  bool success = 1;
  string error = 2;
}

message GetConfigRequest {
}

message GetConfigResponse {
  string config = 1;
  string error = 2;
}

message UpdateConfigRequest {
  string config = 1;
}

message UpdateConfigResponse {
  bool success = 1;
  string error = 2;
}
```

### 5.2 HTTP 接口

| API路径 | 方法 | 功能 | 请求体 (JSON) | 成功响应 (200 OK) |
|---------|------|------|--------------|------------------|
| /api/v1/set | POST | 设置键值对 | `{"key": "...", "value": "...", "ttl": 0}` | `{"success": true, "message": "key set successfully"}` |
| /api/v1/get/{key} | GET | 获取值 | N/A | `{"key": "...", "value": "..."}` |
| /api/v1/delete/{key} | DELETE | 删除键值对 | N/A | `{"success": true, "message": "key deleted successfully"}` |
| /api/v1/scan | GET | 扫描键值对 | N/A (参数通过查询字符串传递: ?prefix=...&limit=...) | `{"prefix": "...", "limit": 100, "count": 5, "results": {...}}` |
| /api/v1/mset | POST | 批量设置键值对 | `{"key_values": {"key1": "value1", "key2": "value2"}}` | `{"success": true, "message": "keys set successfully"}` |
| /api/v1/mget | POST | 批量获取值 | `{"keys": ["key1", "key2"]}` | `{"results": {"key1": "value1", "key2": "value2"}}` |
| /api/v1/mdelete | POST | 批量删除键值对 | `{"keys": ["key1", "key2"]}` | `{"success": true, "message": "keys deleted successfully"}` |
| /api/v1/config | GET | 获取服务配置 | N/A | 配置 JSON 对象 |
| /api/v1/config | POST | 更新服务配置 | 配置 JSON 对象 | `{"success": true, "message": "config updated successfully"}` |
| /api/v1/health | GET | 健康检查 | N/A | `{"status": "ok"}` |

## 6. 数据模型

### 6.1 存储模型

- **键 (Key)**：字符串类型，最大长度 1024 字节
- **值 (Value)**：字符串类型，最大长度 1MB
- **存储格式**：直接使用 RocksDB 的键值存储功能，无需额外转换

## 7. 部署与集成

### 7.1 依赖项

- Golang 1.20+
- RocksDB 7.0+
- gRPC 相关依赖
- Gin 框架

### 7.2 配置项

| 配置项 | 类型 | 默认值 | 描述 |
|--------|------|--------|------|
| rocksdb.path | string | "./data" | RocksDB 数据存储路径 |
| grpc.port | int | 50051 | gRPC 服务端口 |
| http.port | int | 8080 | HTTP 服务端口 |
| rocksdb.options | object | {} | RocksDB 配置选项 |
| value.disk_threshold | int | 1048576 | 大值存储阈值，默认 1MB |
| value.disk_path | string | "./value_data" | 大值存储路径 |
| eviction.enabled | bool | true | 是否启用淘汰机制 |
| eviction.disk_usage_threshold | int | 80 | 磁盘使用阈值，默认 80% |
| eviction.check_interval | int | 60 | 检查间隔，默认 60秒 |
| eviction.batch_size | int | 100 | 批量淘汰大小，默认 100 |
| monitoring.enabled | bool | true | 是否启用监控 |
| monitoring.metrics_path | string | "/metrics" | 指标路径 |
| monitoring.health_path | string | "/api/v1/health" | 健康检查路径 |

## 8. 性能指标

### 8.1 服务层性能

| 测试名称 | 操作/秒 | 平均延迟/操作 |
|---------|---------|--------------|
| Set (单线程) | ~316,732 | ~3,762 ns |
| Get (单线程) | ~1,629,542 | ~783 ns |
| Set (并发) | ~214,846 | ~6,049 ns |
| Get (并发) | ~4,086,961 | ~309.7 ns |
| 混合操作 | ~747,752 | ~1,459 ns |

### 8.2 gRPC客户端性能

| 测试名称 | 操作/秒 | 平均延迟/操作 |
|---------|---------|--------------|
| Set (单线程) | ~20,506 | ~58,621 ns |
| Get (单线程) | ~21,990 | ~49,830 ns |
| Set (并发) | ~65,038 | ~21,667 ns |
| Get (并发) | ~95,284 | ~15,663 ns |
| 混合操作 | ~76,860 | ~15,564 ns |

### 8.3 性能要求

- **吞吐量**：服务层 Get 操作可达 400 万+ 操作/秒
- **延迟**：服务层 Get 操作平均延迟 < 1μs
- **并发**：支持 thousands of concurrent connections

## 9. 监控与维护

### 9.1 日志

- 系统运行日志
- 错误日志
- 性能指标日志

### 9.2 监控

- **服务健康检查**：提供 `/api/v1/health` 接口
- **性能指标监控**：集成 Prometheus 监控，提供以下指标：
  - 操作延迟：`kv_set_latency_seconds`、`kv_get_latency_seconds`、`kv_delete_latency_seconds`、`kv_scan_latency_seconds`
  - 操作计数：`kv_sets_total`、`kv_gets_total`、`kv_deletes_total`、`kv_scans_total`
  - 错误计数：`kv_set_errors_total`、`kv_get_errors_total`、`kv_delete_errors_total`、`kv_scan_errors_total`
  - 配置更新：`kv_config_updates_total`
  - 健康检查：`kv_health_checks_total`、`kv_health_check_latency_seconds`
- **存储使用情况监控**：监控 RocksDB 数据目录大小和磁盘使用率

## 10. 安全考虑

- 接口访问控制
- 数据传输加密
- 防止 DoS 攻击

## 11. 项目计划

1. **需求分析与设计**：完成需求文档和技术方案设计
2. **项目初始化**：搭建项目结构，配置依赖
3. **存储层实现**：封装 RocksDB 操作
4. **接口层实现**：实现 gRPC 和 HTTP 接口
5. **测试与调优**：编写测试用例，性能调优
6. **部署与集成**：编写部署脚本，集成文档

## 12. 验收标准

1. 所有核心功能正常工作
2. gRPC 和 HTTP 接口均能正确响应
3. 性能满足要求
4. 代码质量良好，测试覆盖率高

## 13. 风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| RocksDB 依赖安装复杂 | 部署困难 | 提供详细的依赖安装文档 |
| 高并发下性能问题 | 服务响应慢 | 优化 RocksDB 配置，使用连接池 |
| 数据一致性问题 | 数据丢失 | 合理配置 RocksDB 的持久化策略 |
