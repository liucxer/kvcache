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
	@echo "  make help     - Show this help message"

.PHONY: all build clean run test test-config test-service test-storage test-api test-http test-grpc test-verbose help