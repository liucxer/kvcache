package api_test

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"cachefs/api"
	"cachefs/config"
	"cachefs/proto"
	"cachefs/service"
	"cachefs/storage"
)

var (
	httpServer *api.HTTPServer
	testRouter *gin.Engine
	grpcServer *api.GRPCServer
	grpcClient proto.KeyValueServiceClient
	grpcConn   *grpc.ClientConn
	store      storage.Storage
)

// 初始化测试环境
func TestMain(m *testing.M) {
	// 1. 初始化配置
	cfg := config.DefaultConfig()

	// 2. 删除现有的RocksDB数据目录，确保每次测试都创建新的数据库
	os.RemoveAll(cfg.RocksDB.Path)
	os.RemoveAll(cfg.Value.DiskPath)

	// 创建存储实例
	var err error
	store, err = storage.NewStorage(cfg)
	if err != nil {
		panic("Failed to create storage: " + err.Error())
	}
	defer store.Stop()

	// 创建业务逻辑服务
	kvService := service.NewKVService(store, cfg)

	// 创建HTTP服务器
	httpServer = api.NewHTTPServer(kvService)

	// 获取路由
	testRouter = gin.Default()

	// 设置路由
	testRouter.Static("/web", "./web")
	testRouter.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/web/index.html")
	})
	testRouter.GET("/health", httpServer.HealthCheck)
	testRouter.POST("/api/v1/set", httpServer.Set)
	testRouter.GET("/api/v1/get/:key", httpServer.Get)
	testRouter.DELETE("/api/v1/delete/:key", httpServer.Delete)
	testRouter.GET("/api/v1/scan", httpServer.Scan)
	testRouter.POST("/api/v1/mset", httpServer.MSet)
	testRouter.POST("/api/v1/mget", httpServer.MGet)
	testRouter.POST("/api/v1/mdelete", httpServer.MDelete)
	testRouter.GET("/api/v1/config", httpServer.GetConfig)
	testRouter.POST("/api/v1/config", httpServer.UpdateConfig)
	testRouter.GET("/metrics", gin.WrapH(http.DefaultServeMux))

	// 创建gRPC服务器
	grpcServer = api.NewGRPCServer(kvService)

	// 启动gRPC服务器
	server := grpc.NewServer()
	grpcServer.Register(server)

	// 启动服务器
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		panic("Failed to listen: " + err.Error())
	}

	go func() {
		if err := server.Serve(lis); err != nil {
			panic("Failed to serve gRPC: " + err.Error())
		}
	}()

	// 创建gRPC客户端
	grpcConn, err = grpc.Dial(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic("Failed to dial: " + err.Error())
	}
	defer grpcConn.Close()

	grpcClient = proto.NewKeyValueServiceClient(grpcConn)

	// 运行测试
	m.Run()
}

// 测试健康检查接口
func TestHealthCheck(t *testing.T) {
	// 创建请求
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// 创建响应记录器
	w := httptest.NewRecorder()

	// 处理请求
	testRouter.ServeHTTP(w, req)

	// 检查响应
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	// 解析响应
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// 检查响应内容
	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", response["status"])
	}
}

// 测试设置键值对接口
func TestSet(t *testing.T) {
	// 准备测试数据
	testData := map[string]interface{}{
		"key":   "test-key",
		"value": "test-value",
		"ttl":   0,
	}

	// 转换为JSON
	data, err := json.Marshal(testData)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	// 创建请求
	req, err := http.NewRequest("POST", "/api/v1/set", bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 创建响应记录器
	w := httptest.NewRecorder()

	// 处理请求
	testRouter.ServeHTTP(w, req)

	// 检查响应
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	// 解析响应
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// 检查响应内容
	if !response["success"].(bool) {
		t.Errorf("Expected success true, got %v", response["success"])
	}
}

// 测试获取值接口
func TestGet(t *testing.T) {
	// 先设置一个键值对
	testData := map[string]interface{}{
		"key":   "test-key",
		"value": "test-value",
		"ttl":   0,
	}

	// 转换为JSON
	data, err := json.Marshal(testData)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	// 创建设置请求
	setReq, err := http.NewRequest("POST", "/api/v1/set", bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("Failed to create set request: %v", err)
	}
	setReq.Header.Set("Content-Type", "application/json")

	// 处理设置请求
	setW := httptest.NewRecorder()
	testRouter.ServeHTTP(setW, setReq)

	// 创建获取请求
	getReq, err := http.NewRequest("GET", "/api/v1/get/test-key", nil)
	if err != nil {
		t.Fatalf("Failed to create get request: %v", err)
	}

	// 创建响应记录器
	getW := httptest.NewRecorder()

	// 处理获取请求
	testRouter.ServeHTTP(getW, getReq)

	// 检查响应
	if getW.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, getW.Code)
	}

	// 解析响应
	var response map[string]interface{}
	if err := json.Unmarshal(getW.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// 检查响应内容
	if response["value"] != "test-value" {
		t.Errorf("Expected value 'test-value', got '%v'", response["value"])
	}
}

// 测试删除键值对接口
func TestDelete(t *testing.T) {
	// 先设置一个键值对
	testData := map[string]interface{}{
		"key":   "test-key",
		"value": "test-value",
		"ttl":   0,
	}

	// 转换为JSON
	data, err := json.Marshal(testData)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	// 创建设置请求
	setReq, err := http.NewRequest("POST", "/api/v1/set", bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("Failed to create set request: %v", err)
	}
	setReq.Header.Set("Content-Type", "application/json")

	// 处理设置请求
	setW := httptest.NewRecorder()
	testRouter.ServeHTTP(setW, setReq)

	// 创建删除请求
	deleteReq, err := http.NewRequest("DELETE", "/api/v1/delete/test-key", nil)
	if err != nil {
		t.Fatalf("Failed to create delete request: %v", err)
	}

	// 创建响应记录器
	deleteW := httptest.NewRecorder()

	// 处理删除请求
	testRouter.ServeHTTP(deleteW, deleteReq)

	// 检查响应
	if deleteW.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, deleteW.Code)
	}

	// 解析响应
	var response map[string]interface{}
	if err := json.Unmarshal(deleteW.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// 检查响应内容
	if !response["success"].(bool) {
		t.Errorf("Expected success true, got %v", response["success"])
	}
}

// 测试扫描接口
func TestScan(t *testing.T) {
	// 先设置几个键值对
	testData1 := map[string]interface{}{
		"key":   "test-key-1",
		"value": "test-value-1",
		"ttl":   0,
	}

	testData2 := map[string]interface{}{
		"key":   "test-key-2",
		"value": "test-value-2",
		"ttl":   0,
	}

	// 转换为JSON
	data1, err := json.Marshal(testData1)
	if err != nil {
		t.Fatalf("Failed to marshal test data 1: %v", err)
	}

	data2, err := json.Marshal(testData2)
	if err != nil {
		t.Fatalf("Failed to marshal test data 2: %v", err)
	}

	// 创建设置请求
	setReq1, err := http.NewRequest("POST", "/api/v1/set", bytes.NewBuffer(data1))
	if err != nil {
		t.Fatalf("Failed to create set request 1: %v", err)
	}
	setReq1.Header.Set("Content-Type", "application/json")

	setReq2, err := http.NewRequest("POST", "/api/v1/set", bytes.NewBuffer(data2))
	if err != nil {
		t.Fatalf("Failed to create set request 2: %v", err)
	}
	setReq2.Header.Set("Content-Type", "application/json")

	// 处理设置请求
	setW1 := httptest.NewRecorder()
	testRouter.ServeHTTP(setW1, setReq1)

	setW2 := httptest.NewRecorder()
	testRouter.ServeHTTP(setW2, setReq2)

	// 创建扫描请求
	scanReq, err := http.NewRequest("GET", "/api/v1/scan?prefix=test", nil)
	if err != nil {
		t.Fatalf("Failed to create scan request: %v", err)
	}

	// 创建响应记录器
	scanW := httptest.NewRecorder()

	// 处理扫描请求
	testRouter.ServeHTTP(scanW, scanReq)

	// 检查响应
	if scanW.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, scanW.Code)
	}

	// 解析响应
	var response map[string]interface{}
	if err := json.Unmarshal(scanW.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// 检查响应内容
	results, ok := response["results"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected results to be a map[string]interface{}")
	}

	if len(results) < 2 {
		t.Errorf("Expected at least 2 results, got %d", len(results))
	}
}

// 测试批量设置接口
func TestMSet(t *testing.T) {
	// 准备测试数据
	testData := map[string]interface{}{
		"kvs": map[string]interface{}{
			"batch-key-1": "batch-value-1",
			"batch-key-2": "batch-value-2",
		},
		"ttl": 0,
	}

	// 转换为JSON
	data, err := json.Marshal(testData)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	// 创建请求
	req, err := http.NewRequest("POST", "/api/v1/mset", bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 创建响应记录器
	w := httptest.NewRecorder()

	// 处理请求
	testRouter.ServeHTTP(w, req)

	// 检查响应
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	// 解析响应
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// 检查响应内容
	if !response["success"].(bool) {
		t.Errorf("Expected success true, got %v", response["success"])
	}

	if response["count"].(float64) != 2 {
		t.Errorf("Expected count 2, got %v", response["count"])
	}
}

// 测试批量获取接口
func TestMGet(t *testing.T) {
	// 先批量设置几个键值对
	testData := map[string]interface{}{
		"kvs": map[string]interface{}{
			"batch-key-1": "batch-value-1",
			"batch-key-2": "batch-value-2",
		},
		"ttl": 0,
	}

	// 转换为JSON
	data, err := json.Marshal(testData)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	// 创建设置请求
	setReq, err := http.NewRequest("POST", "/api/v1/mset", bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("Failed to create set request: %v", err)
	}
	setReq.Header.Set("Content-Type", "application/json")

	// 处理设置请求
	setW := httptest.NewRecorder()
	testRouter.ServeHTTP(setW, setReq)

	// 准备批量获取测试数据
	mgetData := map[string]interface{}{
		"keys": []string{"batch-key-1", "batch-key-2"},
	}

	// 转换为JSON
	mgetDataBytes, err := json.Marshal(mgetData)
	if err != nil {
		t.Fatalf("Failed to marshal mget data: %v", err)
	}

	// 创建批量获取请求
	mgetReq, err := http.NewRequest("POST", "/api/v1/mget", bytes.NewBuffer(mgetDataBytes))
	if err != nil {
		t.Fatalf("Failed to create mget request: %v", err)
	}
	mgetReq.Header.Set("Content-Type", "application/json")

	// 创建响应记录器
	mgetW := httptest.NewRecorder()

	// 处理批量获取请求
	testRouter.ServeHTTP(mgetW, mgetReq)

	// 检查响应
	if mgetW.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, mgetW.Code)
	}

	// 解析响应
	var response map[string]interface{}
	if err := json.Unmarshal(mgetW.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// 检查响应内容
	results, ok := response["results"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected results to be a map[string]interface{}")
	}

	if len(results) < 2 {
		t.Errorf("Expected at least 2 results, got %d", len(results))
	}
}

// 测试批量删除接口
func TestMDelete(t *testing.T) {
	// 先批量设置几个键值对
	testData := map[string]interface{}{
		"kvs": map[string]interface{}{
			"batch-key-1": "batch-value-1",
			"batch-key-2": "batch-value-2",
		},
		"ttl": 0,
	}

	// 转换为JSON
	data, err := json.Marshal(testData)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	// 创建设置请求
	setReq, err := http.NewRequest("POST", "/api/v1/mset", bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("Failed to create set request: %v", err)
	}
	setReq.Header.Set("Content-Type", "application/json")

	// 处理设置请求
	setW := httptest.NewRecorder()
	testRouter.ServeHTTP(setW, setReq)

	// 准备批量删除测试数据
	mdeleteData := map[string]interface{}{
		"keys": []string{"batch-key-1", "batch-key-2"},
	}

	// 转换为JSON
	mdeleteDataBytes, err := json.Marshal(mdeleteData)
	if err != nil {
		t.Fatalf("Failed to marshal mdelete data: %v", err)
	}

	// 创建批量删除请求
	mdeleteReq, err := http.NewRequest("POST", "/api/v1/mdelete", bytes.NewBuffer(mdeleteDataBytes))
	if err != nil {
		t.Fatalf("Failed to create mdelete request: %v", err)
	}
	mdeleteReq.Header.Set("Content-Type", "application/json")

	// 创建响应记录器
	mdeleteW := httptest.NewRecorder()

	// 处理批量删除请求
	testRouter.ServeHTTP(mdeleteW, mdeleteReq)

	// 检查响应
	if mdeleteW.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, mdeleteW.Code)
	}

	// 解析响应
	var response map[string]interface{}
	if err := json.Unmarshal(mdeleteW.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// 检查响应内容
	if !response["success"].(bool) {
		t.Errorf("Expected success true, got %v", response["success"])
	}

	if response["count"].(float64) != 2 {
		t.Errorf("Expected count 2, got %v", response["count"])
	}
}

// 测试获取配置接口
func TestGetConfig(t *testing.T) {
	// 创建请求
	req, err := http.NewRequest("GET", "/api/v1/config", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// 创建响应记录器
	w := httptest.NewRecorder()

	// 处理请求
	testRouter.ServeHTTP(w, req)

	// 检查响应
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	// 解析响应
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// 检查响应内容
	if response["rocksdb"] == nil {
		t.Errorf("Expected rocksdb config to be present")
	}
}

// 测试更新配置接口
func TestUpdateConfig(t *testing.T) {
	// 准备测试数据
	testData := map[string]interface{}{
		"rocksdb_path": "./test_data",
	}

	// 转换为JSON
	data, err := json.Marshal(testData)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	// 创建请求
	req, err := http.NewRequest("POST", "/api/v1/config", bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 创建响应记录器
	w := httptest.NewRecorder()

	// 处理请求
	testRouter.ServeHTTP(w, req)

	// 检查响应
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	// 解析响应
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// 检查响应内容
	if !response["success"].(bool) {
		t.Errorf("Expected success true, got %v", response["success"])
	}
}

// 测试大值存储接口
func TestSetLargeValue(t *testing.T) {
	// 准备大值测试数据（1.1MB）
	largeValue := make([]byte, 1100000)
	for i := range largeValue {
		largeValue[i] = byte('a' + i%26)
	}

	testData := map[string]interface{}{
		"key":   "large-test-key",
		"value": string(largeValue),
		"ttl":   0,
	}

	// 转换为JSON
	data, err := json.Marshal(testData)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	// 创建请求
	req, err := http.NewRequest("POST", "/api/v1/set", bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 创建响应记录器
	w := httptest.NewRecorder()

	// 处理请求
	testRouter.ServeHTTP(w, req)

	// 检查响应
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	// 解析响应
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// 检查响应内容
	if !response["success"].(bool) {
		t.Errorf("Expected success true, got %v", response["success"])
	}
}

// 测试错误处理接口
func TestErrorHandling(t *testing.T) {
	// 测试无效的JSON格式
	invalidData := []byte("invalid json")

	// 创建请求
	req, err := http.NewRequest("POST", "/api/v1/set", bytes.NewBuffer(invalidData))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 创建响应记录器
	w := httptest.NewRecorder()

	// 处理请求
	testRouter.ServeHTTP(w, req)

	// 检查响应
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}
}
