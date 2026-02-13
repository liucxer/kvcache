# Makefile for kvcache project

# 定义编译目标
TARGET = kvcache

# 设置环境变量
CGO_CFLAGS = "-I/opt/homebrew/include"
CGO_LDFLAGS = "-L/opt/homebrew/lib  -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd"

# 默认目标
all: build

# 编译目标
build:
	@echo "Building $(TARGET)..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go build -o $(TARGET) .

# 清理目标
clean:
	@echo "Cleaning up..."
	@rm -f $(TARGET)

# 运行目标
run: build
	@echo "Running $(TARGET)..."
	@./$(TARGET)

# 测试目标
test:
	@echo "Running all tests..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test ./...

test-config:
	@echo "Running config tests..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test ./config

test-service:
	@echo "Running service tests..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test ./service

test-storage:
	@echo "Running storage tests..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test ./storage

test-api:
	@echo "Running API tests..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test ./test/api

test-http:
	@echo "Running HTTP API tests..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test ./test/api -run TestHttp

test-grpc:
	@echo "Running gRPC API tests..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test ./test/api -run TestGRPC

test-verbose:
	@echo "Running all tests with verbose output..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test -v ./...

# 性能测试目标
test-performance:
	@echo "Running performance tests..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test -bench=. ./service

test-benchmark-set:
	@echo "Running set benchmark..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test -bench=BenchmarkSet ./service

test-benchmark-get:
	@echo "Running get benchmark..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test -bench=BenchmarkGet ./service

test-benchmark-concurrent:
	@echo "Running concurrent benchmarks..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test -bench=BenchmarkSetConcurrentBenchmarkGetConcurrent ./service

test-benchmark-mixed:
	@echo "Running mixed operations benchmark..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test -bench=BenchmarkMixedOperations ./service

# gRPC性能测试目标
test-grpc-performance:
	@echo "Running gRPC performance tests..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test -bench=BenchmarkGRPC ./test/api

test-grpc-benchmark-set:
	@echo "Running gRPC set benchmark..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test -bench=BenchmarkGRPCSet ./test/api

test-grpc-benchmark-get:
	@echo "Running gRPC get benchmark..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test -bench=BenchmarkGRPCGet ./test/api

test-grpc-benchmark-concurrent:
	@echo "Running gRPC concurrent benchmarks..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test -bench=BenchmarkGRPCSetConcurrentBenchmarkGRPCGetConcurrent ./test/api

test-grpc-benchmark-mixed:
	@echo "Running gRPC mixed operations benchmark..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test -bench=BenchmarkGRPCMixedOperations ./test/api

# 代码覆盖率目标
test-coverage:
	@echo "Running tests with coverage..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test ./... -coverprofile=coverage.out
	@go tool cover -func=coverage.out

test-coverage-html:
	@echo "Running tests with coverage and generating HTML report..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test ./... -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "HTML coverage report generated: coverage.html"

test-coverage-config:
	@echo "Running config tests with coverage..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test ./config -coverprofile=coverage.out
	@go tool cover -func=coverage.out

test-coverage-service:
	@echo "Running service tests with coverage..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test ./service -coverprofile=coverage.out
	@go tool cover -func=coverage.out

test-coverage-storage:
	@echo "Running storage tests with coverage..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test ./storage -coverprofile=coverage.out
	@go tool cover -func=coverage.out

test-coverage-api:
	@echo "Running API tests with coverage..."
	@CGO_CFLAGS=$(CGO_CFLAGS) CGO_LDFLAGS=$(CGO_LDFLAGS) go test ./test/api -coverprofile=coverage.out
	@go tool cover -func=coverage.out

# 帮助目标
help:
	@echo "Makefile for kvcache project"
	@echo "Usage:"
	@echo "  make          - Build the project"
	@echo "  make build    - Build the project"
	@echo "  make clean    - Clean up built files"
	@echo "  make run      - Build and run the project"
	@echo "  make test     - Run all tests"
	@echo "  make test-config - Run config tests"
	@echo "  make test-service - Run service tests"
	@echo "  make test-storage - Run storage tests"
	@echo "  make test-api - Run API tests"
	@echo "  make test-http - Run HTTP API tests"
	@echo "  make test-grpc - Run gRPC API tests"
	@echo "  make test-verbose - Run all tests with verbose output"
	@echo "  make test-performance - Run all performance tests"
	@echo "  make test-benchmark-set - Run set benchmark"
	@echo "  make test-benchmark-get - Run get benchmark"
	@echo "  make test-benchmark-concurrent - Run concurrent benchmarks"
	@echo "  make test-benchmark-mixed - Run mixed operations benchmark"
	@echo "  make test-grpc-performance - Run all gRPC performance tests"
	@echo "  make test-grpc-benchmark-set - Run gRPC set benchmark"
	@echo "  make test-grpc-benchmark-get - Run gRPC get benchmark"
	@echo "  make test-grpc-benchmark-concurrent - Run gRPC concurrent benchmarks"
	@echo "  make test-grpc-benchmark-mixed - Run gRPC mixed operations benchmark"
	@echo "  make test-coverage - Run all tests with coverage"
	@echo "  make test-coverage-html - Run tests with coverage and generate HTML report"
	@echo "  make test-coverage-config - Run config tests with coverage"
	@echo "  make test-coverage-service - Run service tests with coverage"
	@echo "  make test-coverage-storage - Run storage tests with coverage"
	@echo "  make test-coverage-api - Run API tests with coverage"
	@echo "  make help     - Show this help message"

.PHONY: all build clean run test test-config test-service test-storage test-api test-http test-grpc test-verbose test-performance test-benchmark-set test-benchmark-get test-benchmark-concurrent test-benchmark-mixed test-grpc-performance test-grpc-benchmark-set test-grpc-benchmark-get test-grpc-benchmark-concurrent test-grpc-benchmark-mixed test-coverage test-coverage-html test-coverage-config test-coverage-service test-coverage-storage test-coverage-api help