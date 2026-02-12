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

# 帮助目标
help:
	@echo "Makefile for kvcache project"
	@echo "Usage:"
	@echo "  make          - Build the project"
	@echo "  make build    - Build the project"
	@echo "  make clean    - Clean up built files"
	@echo "  make run      - Build and run the project"
	@echo "  make help     - Show this help message"

.PHONY: all build clean run help