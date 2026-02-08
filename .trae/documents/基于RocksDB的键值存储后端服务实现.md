# 基于RocksDB的键值存储后端服务实现计划

## 实现步骤

### 1. 项目初始化
- 创建基本目录结构（api、service、storage、config、metrics、utils、proto）
- 初始化Go模块，添加依赖项
- 创建主入口文件 main.go

### 2. 配置管理模块
- 实现配置结构体和默认配置
- 实现配置的加载、存储和更新功能
- 配置存储到RocksDB中，键为 `global.config`

### 3. Protobuf 定义
- 创建 proto 文件，定义 gRPC 服务和消息类型
- 生成 Go 代码

### 4. 存储层实现
- **RocksDB 封装**：实现基本的键值操作
- **磁盘存储**：实现大 value 的磁盘存储
- **存储接口**：统一存储操作接口
- **创建时间管理**：实现键的创建时间记录
- **淘汰管理器**：实现基于 FIFO 的淘汰机制

### 5. 业务逻辑层
- 实现核心功能：Set、Get、Delete、Scan、MSet、MGet、MDelete
- 实现配置相关功能：GetConfig、UpdateConfig
- 处理存储位置选择和淘汰逻辑

### 6. 接口层实现
- **gRPC 接口**：实现高性能 RPC 接口
- **HTTP 接口**：实现调试用 HTTP 接口
- **健康检查**：实现服务健康状态检查

### 7. 监控系统
- 实现性能指标收集
- 实现存储监控
- 实现淘汰统计
- 集成 Prometheus 客户端

### 8. 主程序实现
- 初始化各个模块
- 启动 gRPC 和 HTTP 服务
- 管理服务生命周期

### 9. 测试和验证
- 编译项目
- 启动服务
- 测试核心功能
- 验证监控系统

## 技术要点

1. **存储策略**：根据 value 大小自动选择存储位置，大 value 存储到磁盘
2. **淘汰机制**：基于 FIFO 算法，支持配置开关
3. **双接口**：同时提供 gRPC（业务使用）和 HTTP（调试使用）接口
4. **监控系统**：完善的健康检查、性能指标、存储监控和淘汰统计
5. **配置管理**：提供接口查询和修改当前配置

## 编译命令

使用指定的编译命令：
```bash
CGO_CFLAGS="-I/opt/homebrew/include" CGO_LDFLAGS="-L/opt/homebrew/lib  -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd" go build .
```