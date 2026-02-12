# 分析项目测试用例

## 测试文件分布
- `config/config_test.go` - 配置相关测试
- `service/service_test.go` - 服务层测试
- `storage/storage_test.go` - 存储层测试
- `test/api/grpc_test.go` - gRPC API测试
- `test/api/http_test.go` - HTTP API测试

## 测试内容
- **配置测试**：默认配置、JSON解析和转换
- **存储测试**：基本的设置/获取、批量操作、扫描、磁盘存储、淘汰机制
- **服务测试**：服务创建、基本操作、批量操作、扫描、配置管理、健康检查、错误处理
- **API测试**：HTTP和gRPC接口的各种操作

## 测试特点
- 每个测试都有独立的测试环境，会在测试前清理数据目录
- 测试覆盖了正常流程和错误处理
- 测试包含了基本操作和边界情况（如大值存储）
- HTTP和gRPC测试共享测试环境设置

## 计划
1. 在Makefile中添加测试相关的命令：
   - `test` - 运行所有测试
   - `test-config` - 运行配置相关测试
   - `test-service` - 运行服务层测试
   - `test-storage` - 运行存储层测试
   - `test-api` - 运行API测试
   - `test-http` - 运行HTTP API测试
   - `test-grpc` - 运行gRPC API测试
   - `test-verbose` - 运行所有测试并显示详细输出

2. 确保测试命令使用正确的环境变量和参数，以便测试可以正常运行