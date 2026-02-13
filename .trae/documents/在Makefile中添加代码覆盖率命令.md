# 在Makefile中添加代码覆盖率命令

## 目标
在现有的Makefile中添加代码覆盖率相关的命令，以便用户可以方便地查看项目的代码覆盖率情况。

## 实现方案

### 1. 添加代码覆盖率命令

在Makefile中添加以下命令：

- **test-coverage**: 运行所有测试并生成文本格式的代码覆盖率报告
- **test-coverage-html**: 运行所有测试并生成HTML格式的代码覆盖率报告
- **test-coverage-config**: 只测试config包的代码覆盖率
- **test-coverage-service**: 只测试service包的代码覆盖率
- **test-coverage-storage**: 只测试storage包的代码覆盖率
- **test-coverage-api**: 只测试API相关的代码覆盖率

### 2. 命令实现细节

所有命令将使用现有的CGO环境变量设置，确保与其他测试命令保持一致：

```makefile
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
```

### 3. 更新帮助信息

在`help`目标中添加新命令的说明，以便用户可以通过`make help`查看所有可用的命令。

### 4. 更新PHONY目标

在Makefile末尾的`.PHONY`声明中添加新的命令，确保它们被正确识别为伪目标。

## 预期效果

添加这些命令后，用户可以：

1. 使用`make test-coverage`快速查看整个项目的代码覆盖率
2. 使用`make test-coverage-html`生成详细的HTML格式覆盖率报告，查看具体哪些代码行被覆盖
3. 使用针对特定包的覆盖率命令，如`make test-coverage-service`，只查看相关包的覆盖率情况

这些命令将帮助用户更好地了解项目的测试覆盖情况，从而提高代码质量。