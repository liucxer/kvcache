package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"kvcache/service"
)

// HTTPServer HTTP服务器
type HTTPServer struct {
	service *service.KVService
	router  *gin.Engine
}

// NewHTTPServer 创建新的HTTP服务器实例
func NewHTTPServer(service *service.KVService) *HTTPServer {
	router := gin.Default()
	server := &HTTPServer{
		service: service,
		router:  router,
	}

	server.setupRoutes()
	return server
}

// setupRoutes 设置路由
func (s *HTTPServer) setupRoutes() {
	// 静态文件服务
	s.router.Static("/web", "./web")
	s.router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/web/index.html")
	})

	// 健康检查
	s.router.GET("/health", s.HealthCheck)

	// 键值操作
	s.router.POST("/api/v1/set", s.Set)
	s.router.GET("/api/v1/get/:key", s.Get)
	s.router.DELETE("/api/v1/delete/:key", s.Delete)
	s.router.GET("/api/v1/scan", s.Scan)
	s.router.POST("/api/v1/mset", s.MSet)
	s.router.POST("/api/v1/mget", s.MGet)
	s.router.POST("/api/v1/mdelete", s.MDelete)

	// 配置管理
	s.router.GET("/api/v1/config", s.GetConfig)
	s.router.POST("/api/v1/config", s.UpdateConfig)

	// 监控指标
	s.router.GET("/metrics", gin.WrapH(http.DefaultServeMux))
}

// Run 启动HTTP服务器
func (s *HTTPServer) Run(addr string) error {
	return s.router.Run(addr)
}

// HealthCheck 健康检查
func (s *HTTPServer) HealthCheck(c *gin.Context) {
	err := s.service.HealthCheck(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "unhealthy",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"message": "service is running",
	})
}

// Set 设置键值对
func (s *HTTPServer) Set(c *gin.Context) {
	var req struct {
		Key   string `json:"key" binding:"required"`
		Value string `json:"value" binding:"required"`
		TTL   int64  `json:"ttl"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request: " + err.Error(),
		})
		return
	}

	var ttl time.Duration
	if req.TTL > 0 {
		ttl = time.Duration(req.TTL) * time.Second
	}

	err := s.service.Set(c.Request.Context(), req.Key, []byte(req.Value), ttl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to set: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "key set successfully",
	})
}

// Get 获取值
func (s *HTTPServer) Get(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "key is required",
		})
		return
	}

	value, err := s.service.Get(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "key not found: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"key":   key,
		"value": string(value),
	})
}

// Delete 删除键值对
func (s *HTTPServer) Delete(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "key is required",
		})
		return
	}

	err := s.service.Delete(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to delete: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "key deleted successfully",
	})
}

// Scan 扫描键值对
func (s *HTTPServer) Scan(c *gin.Context) {
	prefix := c.Query("prefix")
	limitStr := c.DefaultQuery("limit", "100")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	results, err := s.service.Scan(c.Request.Context(), prefix, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to scan: " + err.Error(),
		})
		return
	}

	// 将map[string][]byte转换为map[string]string
	stringResults := make(map[string]string, len(results))
	for k, v := range results {
		stringResults[k] = string(v)
	}

	c.JSON(http.StatusOK, gin.H{
		"prefix":  prefix,
		"limit":   limit,
		"count":   len(results),
		"results": stringResults,
	})
}

// MSet 批量设置键值对
func (s *HTTPServer) MSet(c *gin.Context) {
	var req struct {
		Kvs map[string]string `json:"kvs" binding:"required"`
		TTL int64             `json:"ttl"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request: " + err.Error(),
		})
		return
	}

	if len(req.Kvs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "kvs cannot be empty",
		})
		return
	}

	var ttl time.Duration
	if req.TTL > 0 {
		ttl = time.Duration(req.TTL) * time.Second
	}

	// 将map[string]string转换为map[string][]byte
	keyValues := make(map[string][]byte, len(req.Kvs))
	for k, v := range req.Kvs {
		keyValues[k] = []byte(v)
	}

	err := s.service.MSet(c.Request.Context(), keyValues, ttl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to mset: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "keys set successfully",
		"count":   len(req.Kvs),
	})
}

// MGet 批量获取值
func (s *HTTPServer) MGet(c *gin.Context) {
	var req struct {
		Keys []string `json:"keys" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request: " + err.Error(),
		})
		return
	}

	if len(req.Keys) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "keys cannot be empty",
		})
		return
	}

	results, err := s.service.MGet(c.Request.Context(), req.Keys)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to mget: " + err.Error(),
		})
		return
	}

	// 将map[string][]byte转换为map[string]string
	stringResults := make(map[string]string, len(results))
	for k, v := range results {
		stringResults[k] = string(v)
	}

	c.JSON(http.StatusOK, gin.H{
		"keys":    req.Keys,
		"count":   len(results),
		"results": stringResults,
	})
}

// MDelete 批量删除键值对
func (s *HTTPServer) MDelete(c *gin.Context) {
	var req struct {
		Keys []string `json:"keys" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request: " + err.Error(),
		})
		return
	}

	if len(req.Keys) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "keys cannot be empty",
		})
		return
	}

	err := s.service.MDelete(c.Request.Context(), req.Keys)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to mdelete: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "keys deleted successfully",
		"count":   len(req.Keys),
	})
}

// GetConfig 获取配置
func (s *HTTPServer) GetConfig(c *gin.Context) {
	config, err := s.service.GetConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get config: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, config)
}

// UpdateConfig 更新配置
func (s *HTTPServer) UpdateConfig(c *gin.Context) {
	var req struct {
		RocksDBPath           string  `json:"rocksdb_path"`
		DiskStorePath         string  `json:"disk_store_path"`
		LargeValueSize        int     `json:"large_value_size"`
		MaxDiskUsage          float64 `json:"max_disk_usage"`
		EvictionCheckInterval int     `json:"eviction_check_interval"`
		EvictionBatchSize     int     `json:"eviction_batch_size"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request: " + err.Error(),
		})
		return
	}

	config, err := s.service.GetConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get current config: " + err.Error(),
		})
		return
	}

	// 更新配置字段
	if req.RocksDBPath != "" {
		config.RocksDB.Path = req.RocksDBPath
	}
	if req.DiskStorePath != "" {
		config.Value.DiskPath = req.DiskStorePath
	}
	if req.LargeValueSize > 0 {
		config.Value.DiskThreshold = req.LargeValueSize
	}
	if req.MaxDiskUsage > 0 {
		config.Eviction.DiskUsageThreshold = req.MaxDiskUsage
	}
	if req.EvictionCheckInterval > 0 {
		config.Eviction.CheckInterval = req.EvictionCheckInterval
	}
	if req.EvictionBatchSize > 0 {
		config.Eviction.BatchSize = req.EvictionBatchSize
	}

	err = s.service.UpdateConfig(c.Request.Context(), config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to update config: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "config updated successfully",
		"config":  config,
	})
}
