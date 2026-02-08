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

### 2.2 技术需求

1. **存储引擎**：使用 RocksDB 作为底层存储引擎
2. **接口类型**：同时提供 gRPC 和 HTTP 接口
3. **编程语言**：使用 Golang 实现
4. **性能要求**：高性能、低延迟
5. **可靠性**：数据持久化存储

## 3. 技术栈

| 技术 | 版本 | 用途 |
|------|------|------|
| Golang | 1.20+ | 主要开发语言 |
| RocksDB | 7.0+ | 底层存储引擎 |
| gRPC | 1.50+ | 高性能 RPC 接口 |
| Protocol Buffers | 3.0+ | 数据序列化 |
| Gin | 1.9+ | HTTP 接口框架 |

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
```

### 5.2 HTTP 接口

| API路径 | 方法 | 功能 | 请求体 (JSON) | 成功响应 (200 OK) |
|---------|------|------|--------------|------------------|
| /api/v1/set | POST | 设置键值对 | `{"key": "...", "value": "..."}` | `{"success": true, "error": ""}` |
| /api/v1/get | GET | 获取值 | N/A (参数通过查询字符串传递: ?key=...) | `{"value": "...", "found": true, "error": ""}` |
| /api/v1/delete | DELETE | 删除键值对 | N/A (参数通过查询字符串传递: ?key=...) | `{"success": true, "error": ""}` |
| /api/v1/scan/keys | GET | 扫描键前缀 | N/A (参数通过查询字符串传递: ?prefix=...) | `{"keys": [...], "error": ""}` |
| /api/v1/scan/keyvalues | GET | 扫描键值对 | N/A (参数通过查询字符串传递: ?prefix=...) | `{"key_values": {...}, "error": ""}` |

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

## 8. 性能指标

- **吞吐量**：每秒处理 thousands of requests
- **延迟**：P99 延迟 < 1ms
- **并发**：支持 thousands of concurrent connections

## 9. 监控与维护

### 9.1 日志

- 系统运行日志
- 错误日志
- 性能指标日志

### 9.2 监控

- 服务健康检查
- 性能指标监控
- 存储使用情况监控

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
