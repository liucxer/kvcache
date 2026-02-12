package api_test

import (
	"context"
	"testing"

	"kvcache/proto"
)

// 全局变量已在http_test.go中声明

// 测试设置键值对接口
func TestGRPCSet(t *testing.T) {

	// 创建请求
	req := &proto.SetRequest{
		Key:   []byte("grpc-test-key"),
		Value: []byte("grpc-test-value"),
	}

	// 发送请求
	resp, err := grpcClient.Set(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed to set: %v", err)
	}

	// 检查响应
	if !resp.Success {
		t.Errorf("Expected success true, got %v", resp.Success)
	}
}

// 测试获取值接口
func TestGRPCGet(t *testing.T) {

	// 先设置一个键值对
	setReq := &proto.SetRequest{
		Key:   []byte("grpc-test-key"),
		Value: []byte("grpc-test-value"),
	}

	_, err := grpcClient.Set(context.Background(), setReq)
	if err != nil {
		t.Fatalf("Failed to set: %v", err)
	}

	// 创建获取请求
	getReq := &proto.GetRequest{
		Key: []byte("grpc-test-key"),
	}

	// 发送请求
	getResp, err := grpcClient.Get(context.Background(), getReq)
	if err != nil {
		t.Fatalf("Failed to get: %v", err)
	}

	// 检查响应
	if !getResp.Found {
		t.Errorf("Expected found true, got %v", getResp.Found)
	}

	if string(getResp.Value) != "grpc-test-value" {
		t.Errorf("Expected value 'grpc-test-value', got '%s'", string(getResp.Value))
	}
}

// 测试删除键值对接口
func TestGRPCDelete(t *testing.T) {

	// 先设置一个键值对
	setReq := &proto.SetRequest{
		Key:   []byte("grpc-test-key"),
		Value: []byte("grpc-test-value"),
	}

	_, err := grpcClient.Set(context.Background(), setReq)
	if err != nil {
		t.Fatalf("Failed to set: %v", err)
	}

	// 创建删除请求
	deleteReq := &proto.DeleteRequest{
		Key: []byte("grpc-test-key"),
	}

	// 发送请求
	deleteResp, err := grpcClient.Delete(context.Background(), deleteReq)
	if err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}

	// 检查响应
	if !deleteResp.Success {
		t.Errorf("Expected success true, got %v", deleteResp.Success)
	}
}

// 测试扫描键值对接口
func TestGRPCScanKeyValues(t *testing.T) {

	// 先设置几个键值对
	setReq1 := &proto.SetRequest{
		Key:   []byte("grpc-test-key-1"),
		Value: []byte("grpc-test-value-1"),
	}

	setReq2 := &proto.SetRequest{
		Key:   []byte("grpc-test-key-2"),
		Value: []byte("grpc-test-value-2"),
	}

	_, err := grpcClient.Set(context.Background(), setReq1)
	if err != nil {
		t.Fatalf("Failed to set 1: %v", err)
	}

	_, err = grpcClient.Set(context.Background(), setReq2)
	if err != nil {
		t.Fatalf("Failed to set 2: %v", err)
	}

	// 创建扫描请求
	scanReq := &proto.ScanRequest{
		Prefix: []byte("grpc-test"),
	}

	// 发送请求
	scanResp, err := grpcClient.ScanKeyValues(context.Background(), scanReq)
	if err != nil {
		t.Fatalf("Failed to scan: %v", err)
	}

	// 检查响应
	if scanResp.Error != "" {
		t.Errorf("Expected no error, got '%s'", scanResp.Error)
	}

	if len(scanResp.KeyValues) < 2 {
		t.Errorf("Expected at least 2 key-values, got %d", len(scanResp.KeyValues))
	}
}

// 测试扫描键接口
func TestGRPCScanKeys(t *testing.T) {

	// 先设置几个键值对
	setReq1 := &proto.SetRequest{
		Key:   []byte("grpc-test-key-1"),
		Value: []byte("grpc-test-value-1"),
	}

	setReq2 := &proto.SetRequest{
		Key:   []byte("grpc-test-key-2"),
		Value: []byte("grpc-test-value-2"),
	}

	_, err := grpcClient.Set(context.Background(), setReq1)
	if err != nil {
		t.Fatalf("Failed to set 1: %v", err)
	}

	_, err = grpcClient.Set(context.Background(), setReq2)
	if err != nil {
		t.Fatalf("Failed to set 2: %v", err)
	}

	// 创建扫描请求
	scanReq := &proto.ScanRequest{
		Prefix: []byte("grpc-test"),
	}

	// 发送请求
	scanResp, err := grpcClient.ScanKeys(context.Background(), scanReq)
	if err != nil {
		t.Fatalf("Failed to scan: %v", err)
	}

	// 检查响应
	if scanResp.Error != "" {
		t.Errorf("Expected no error, got '%s'", scanResp.Error)
	}

	if len(scanResp.Keys) < 2 {
		t.Errorf("Expected at least 2 keys, got %d", len(scanResp.Keys))
	}
}

// 测试批量设置接口
func TestGRPCMSet(t *testing.T) {

	// 创建请求
	req := &proto.MSetRequest{
		KeyValues: map[string][]byte{
			"grpc-batch-key-1": []byte("grpc-batch-value-1"),
			"grpc-batch-key-2": []byte("grpc-batch-value-2"),
		},
	}

	// 发送请求
	resp, err := grpcClient.MSet(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed to mset: %v", err)
	}

	// 检查响应
	if !resp.Success {
		t.Errorf("Expected success true, got %v", resp.Success)
	}
}

// 测试批量获取接口
func TestGRPCMGet(t *testing.T) {

	// 先批量设置几个键值对
	msetReq := &proto.MSetRequest{
		KeyValues: map[string][]byte{
			"grpc-batch-key-1": []byte("grpc-batch-value-1"),
			"grpc-batch-key-2": []byte("grpc-batch-value-2"),
		},
	}

	_, err := grpcClient.MSet(context.Background(), msetReq)
	if err != nil {
		t.Fatalf("Failed to mset: %v", err)
	}

	// 创建批量获取请求
	mgetReq := &proto.MGetRequest{
		Keys: [][]byte{
			[]byte("grpc-batch-key-1"),
			[]byte("grpc-batch-key-2"),
		},
	}

	// 发送请求
	mgetResp, err := grpcClient.MGet(context.Background(), mgetReq)
	if err != nil {
		t.Fatalf("Failed to mget: %v", err)
	}

	// 检查响应
	if mgetResp.Error != "" {
		t.Errorf("Expected no error, got '%s'", mgetResp.Error)
	}

	if len(mgetResp.KeyValues) < 2 {
		t.Errorf("Expected at least 2 key-values, got %d", len(mgetResp.KeyValues))
	}
}

// 测试批量删除接口
func TestGRPCMDelete(t *testing.T) {

	// 先批量设置几个键值对
	msetReq := &proto.MSetRequest{
		KeyValues: map[string][]byte{
			"grpc-batch-key-1": []byte("grpc-batch-value-1"),
			"grpc-batch-key-2": []byte("grpc-batch-value-2"),
		},
	}

	_, err := grpcClient.MSet(context.Background(), msetReq)
	if err != nil {
		t.Fatalf("Failed to mset: %v", err)
	}

	// 创建批量删除请求
	mdeleteReq := &proto.MDeleteRequest{
		Keys: [][]byte{
			[]byte("grpc-batch-key-1"),
			[]byte("grpc-batch-key-2"),
		},
	}

	// 发送请求
	mdeleteResp, err := grpcClient.MDelete(context.Background(), mdeleteReq)
	if err != nil {
		t.Fatalf("Failed to mdelete: %v", err)
	}

	// 检查响应
	if !mdeleteResp.Success {
		t.Errorf("Expected success true, got %v", mdeleteResp.Success)
	}
}

// 测试获取配置接口
func TestGRPCGetConfig(t *testing.T) {

	// 创建请求
	req := &proto.GetConfigRequest{}

	// 发送请求
	resp, err := grpcClient.GetConfig(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	// 检查响应
	if resp.Error != "" {
		t.Errorf("Expected no error, got '%s'", resp.Error)
	}

	if resp.Config == "" {
		t.Errorf("Expected config to be present")
	}
}

// 测试更新配置接口
func TestGRPCUpdateConfig(t *testing.T) {

	// 创建请求
	req := &proto.UpdateConfigRequest{
		Config: `{
			"rocksdb": {
				"path": "./grpc_test_data"
			}
		}`,
	}

	// 发送请求
	resp, err := grpcClient.UpdateConfig(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	// 检查响应
	if !resp.Success {
		t.Errorf("Expected success true, got %v", resp.Success)
	}
}

// 测试健康检查接口
func TestGRPCHealthCheck(t *testing.T) {

	// 创建健康检查客户端
	healthClient := proto.NewHealthClient(grpcConn)

	// 创建请求
	req := &proto.HealthCheckRequest{}

	// 发送请求
	resp, err := healthClient.Check(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed to health check: %v", err)
	}

	// 检查响应
	if resp.Status != proto.HealthCheckResponse_SERVING {
		t.Errorf("Expected status SERVING, got %v", resp.Status)
	}
}

// 测试大值存储接口
func TestGRPCSetLargeValue(t *testing.T) {

	// 准备大值测试数据（1.1MB）
	largeValue := make([]byte, 1100000)
	for i := range largeValue {
		largeValue[i] = byte('a' + i%26)
	}

	// 创建请求
	req := &proto.SetRequest{
		Key:   []byte("grpc-large-test-key"),
		Value: largeValue,
	}

	// 发送请求
	resp, err := grpcClient.Set(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed to set large value: %v", err)
	}

	// 检查响应
	if !resp.Success {
		t.Errorf("Expected success true, got %v", resp.Success)
	}
}

// 测试获取不存在的键
func TestGRPCGetNonExistentKey(t *testing.T) {

	// 创建获取请求
	req := &proto.GetRequest{
		Key: []byte("non-existent-key"),
	}

	// 发送请求
	resp, err := grpcClient.Get(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed to get non-existent key: %v", err)
	}

	// 检查响应
	if resp.Found {
		t.Errorf("Expected found false for non-existent key, got %v", resp.Found)
	}
}

// 测试删除不存在的键
func TestGRPCDeleteNonExistentKey(t *testing.T) {

	// 创建删除请求
	req := &proto.DeleteRequest{
		Key: []byte("non-existent-key"),
	}

	// 发送请求
	resp, err := grpcClient.Delete(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed to delete non-existent key: %v", err)
	}

	// 检查响应
	if !resp.Success {
		t.Errorf("Expected success true for deleting non-existent key, got %v", resp.Success)
	}
}
