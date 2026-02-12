# 基于gRPC客户端的性能测试用例

## 计划

1. **创建性能测试文件**：在 `test/api` 目录下创建 `grpc_performance_test.go` 文件，用于测试 gRPC 客户端的性能。

2. **编写测试用例**：
   - `BenchmarkGRPCSet` - 测试 gRPC 客户端的单线程 set 操作性能
   - `BenchmarkGRPCGet` - 测试 gRPC 客户端的单线程 get 操作性能
   - `BenchmarkGRPCSetConcurrent` - 测试 gRPC 客户端的并发 set 操作性能
   - `BenchmarkGRPCGetConcurrent` - 测试 gRPC 客户端的并发 get 操作性能
   - `BenchmarkGRPCMixedOperations` - 测试 gRPC 客户端的混合操作性能

3. **更新 Makefile**：添加 gRPC 性能测试相关的命令，以便用户可以方便地运行这些测试。

4. **运行测试**：启动服务并运行性能测试，评估 gRPC 客户端的性能。

## 实现细节

1. **测试环境设置**：
   - 使用 `TestMain` 函数在测试前启动服务
   - 服务启动后，创建 gRPC 客户端连接
   - 测试完成后，关闭客户端连接和服务

2. **测试用例设计**：
   - 生成随机的键值对用于测试
   - 预热阶段：预先设置一些键值对，用于 get 操作测试
   - 测试阶段：执行 set 或 get 操作，并测量性能
   - 并发测试：使用 `b.RunParallel` 执行并发操作

3. **性能指标**：
   - 操作次数/秒
   - 平均耗时/操作

4. **Makefile 命令**：
   - `make test-grpc-performance` - 运行所有 gRPC 性能测试
   - `make test-grpc-benchmark-set` - 运行 gRPC set 操作性能测试
   - `make test-grpc-benchmark-get` - 运行 gRPC get 操作性能测试
   - `make test-grpc-benchmark-concurrent` - 运行 gRPC 并发操作性能测试
   - `make test-grpc-benchmark-mixed` - 运行 gRPC 混合操作性能测试