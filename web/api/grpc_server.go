package api

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc"

	"cachefs/proto"
	"cachefs/service"
)

// GRPCServer gRPC服务器
type GRPCServer struct {
	proto.UnimplementedKeyValueServiceServer
	proto.UnimplementedHealthServer
	service *service.KVService
}

// NewGRPCServer 创建新的gRPC服务器实例
func NewGRPCServer(service *service.KVService) *GRPCServer {
	return &GRPCServer{
		service: service,
	}
}

// Register 注册gRPC服务
func (s *GRPCServer) Register(srv *grpc.Server) {
	proto.RegisterKeyValueServiceServer(srv, s)
	proto.RegisterHealthServer(srv, s)
}

// Set 设置键值对
func (s *GRPCServer) Set(ctx context.Context, req *proto.SetRequest) (*proto.SetResponse, error) {
	if len(req.Key) == 0 {
		return &proto.SetResponse{Success: false, Error: "empty key"}, nil
	}

	err := s.service.Set(ctx, string(req.Key), req.Value, 0)
	if err != nil {
		return &proto.SetResponse{Success: false, Error: err.Error()}, nil
	}

	return &proto.SetResponse{Success: true}, nil
}

// Get 获取值
func (s *GRPCServer) Get(ctx context.Context, req *proto.GetRequest) (*proto.GetResponse, error) {
	if len(req.Key) == 0 {
		return &proto.GetResponse{Found: false, Error: "empty key"}, nil
	}

	value, err := s.service.Get(ctx, string(req.Key))
	if err != nil {
		return &proto.GetResponse{Found: false, Error: err.Error()}, nil
	}

	return &proto.GetResponse{Value: value, Found: true}, nil
}

// Delete 删除键值对
func (s *GRPCServer) Delete(ctx context.Context, req *proto.DeleteRequest) (*proto.DeleteResponse, error) {
	if len(req.Key) == 0 {
		return &proto.DeleteResponse{Success: false, Error: "empty key"}, nil
	}

	err := s.service.Delete(ctx, string(req.Key))
	if err != nil {
		return &proto.DeleteResponse{Success: false, Error: err.Error()}, nil
	}

	return &proto.DeleteResponse{Success: true}, nil
}

// ScanKeys 扫描键
func (s *GRPCServer) ScanKeys(ctx context.Context, req *proto.ScanRequest) (*proto.ScanKeysResponse, error) {
	results, err := s.service.Scan(ctx, string(req.Prefix), 100)
	if err != nil {
		return &proto.ScanKeysResponse{Error: err.Error()}, nil
	}

	// 转换为[]byte类型的keys
	keys := make([][]byte, 0, len(results))
	for k := range results {
		keys = append(keys, []byte(k))
	}

	return &proto.ScanKeysResponse{Keys: keys}, nil
}

// ScanKeyValues 扫描键值对
func (s *GRPCServer) ScanKeyValues(ctx context.Context, req *proto.ScanRequest) (*proto.ScanKeyValuesResponse, error) {
	results, err := s.service.Scan(ctx, string(req.Prefix), 100)
	if err != nil {
		return &proto.ScanKeyValuesResponse{Error: err.Error()}, nil
	}

	return &proto.ScanKeyValuesResponse{KeyValues: results}, nil
}

// MSet 批量设置键值对
func (s *GRPCServer) MSet(ctx context.Context, req *proto.MSetRequest) (*proto.MSetResponse, error) {
	if len(req.KeyValues) == 0 {
		return &proto.MSetResponse{Success: false, Error: "empty key-value pairs"}, nil
	}

	err := s.service.MSet(ctx, req.KeyValues, 0)
	if err != nil {
		return &proto.MSetResponse{Success: false, Error: err.Error()}, nil
	}

	return &proto.MSetResponse{Success: true}, nil
}

// MGet 批量获取值
func (s *GRPCServer) MGet(ctx context.Context, req *proto.MGetRequest) (*proto.MGetResponse, error) {
	if len(req.Keys) == 0 {
		return &proto.MGetResponse{Error: "empty keys"}, nil
	}

	// 转换为[]string类型的keys
	keys := make([]string, len(req.Keys))
	for i, key := range req.Keys {
		keys[i] = string(key)
	}

	results, err := s.service.MGet(ctx, keys)
	if err != nil {
		return &proto.MGetResponse{Error: err.Error()}, nil
	}

	return &proto.MGetResponse{KeyValues: results}, nil
}

// MDelete 批量删除键值对
func (s *GRPCServer) MDelete(ctx context.Context, req *proto.MDeleteRequest) (*proto.MDeleteResponse, error) {
	if len(req.Keys) == 0 {
		return &proto.MDeleteResponse{Success: false, Error: "empty keys"}, nil
	}

	// 转换为[]string类型的keys
	keys := make([]string, len(req.Keys))
	for i, key := range req.Keys {
		keys[i] = string(key)
	}

	err := s.service.MDelete(ctx, keys)
	if err != nil {
		return &proto.MDeleteResponse{Success: false, Error: err.Error()}, nil
	}

	return &proto.MDeleteResponse{Success: true}, nil
}

// GetConfig 获取配置
func (s *GRPCServer) GetConfig(ctx context.Context, req *proto.GetConfigRequest) (*proto.GetConfigResponse, error) {
	config, err := s.service.GetConfig(ctx)
	if err != nil {
		return &proto.GetConfigResponse{Error: err.Error()}, nil
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return &proto.GetConfigResponse{Error: err.Error()}, nil
	}

	return &proto.GetConfigResponse{Config: string(configJSON)}, nil
}

// UpdateConfig 更新配置
func (s *GRPCServer) UpdateConfig(ctx context.Context, req *proto.UpdateConfigRequest) (*proto.UpdateConfigResponse, error) {
	var config struct {
		RocksDB struct {
			Path    string                 `json:"path"`
			Options map[string]interface{} `json:"options"`
		} `json:"rocksdb"`

		Value struct {
			DiskThreshold int    `json:"disk_threshold"`
			DiskPath      string `json:"disk_path"`
		} `json:"value"`

		Eviction struct {
			Enabled            bool    `json:"enabled"`
			DiskUsageThreshold float64 `json:"disk_usage_threshold"`
			CheckInterval      int     `json:"check_interval"`
			BatchSize          int     `json:"batch_size"`
		} `json:"eviction"`
	}

	if err := json.Unmarshal([]byte(req.Config), &config); err != nil {
		return &proto.UpdateConfigResponse{Success: false, Error: err.Error()}, nil
	}

	// 获取当前配置
	currentConfig, err := s.service.GetConfig(ctx)
	if err != nil {
		return &proto.UpdateConfigResponse{Success: false, Error: err.Error()}, nil
	}

	// 更新配置
	if config.RocksDB.Path != "" {
		currentConfig.RocksDB.Path = config.RocksDB.Path
	}
	if config.RocksDB.Options != nil {
		currentConfig.RocksDB.Options = config.RocksDB.Options
	}
	if config.Value.DiskThreshold > 0 {
		currentConfig.Value.DiskThreshold = config.Value.DiskThreshold
	}
	if config.Value.DiskPath != "" {
		currentConfig.Value.DiskPath = config.Value.DiskPath
	}
	if config.Eviction.Enabled {
		currentConfig.Eviction.Enabled = config.Eviction.Enabled
	}
	if config.Eviction.DiskUsageThreshold > 0 {
		currentConfig.Eviction.DiskUsageThreshold = config.Eviction.DiskUsageThreshold
	}
	if config.Eviction.CheckInterval > 0 {
		currentConfig.Eviction.CheckInterval = config.Eviction.CheckInterval
	}
	if config.Eviction.BatchSize > 0 {
		currentConfig.Eviction.BatchSize = config.Eviction.BatchSize
	}

	err = s.service.UpdateConfig(ctx, currentConfig)
	if err != nil {
		return &proto.UpdateConfigResponse{Success: false, Error: err.Error()}, nil
	}

	return &proto.UpdateConfigResponse{Success: true}, nil
}

// Check 健康检查
func (s *GRPCServer) Check(ctx context.Context, req *proto.HealthCheckRequest) (*proto.HealthCheckResponse, error) {
	err := s.service.HealthCheck(ctx)
	if err != nil {
		return &proto.HealthCheckResponse{Status: proto.HealthCheckResponse_NOT_SERVING}, nil
	}

	return &proto.HealthCheckResponse{Status: proto.HealthCheckResponse_SERVING}, nil
}
