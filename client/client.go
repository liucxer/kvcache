package client

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/tikv/client-go/v2/config"
	"github.com/tikv/client-go/v2/rawkv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"kvcache/proto"
)

var (
	ErrKeyNotFound      = errors.New("kvcache: key not found")
)

type Config struct {
	Node              string
	TiKVPD            string
	RefreshInterval   time.Duration
	HeartbeatTimeout  time.Duration
	UsageThreshold    float64
	RouteCacheSize    int64
	MaxRecvMsgSize    int
	CACert            string
	ClientCert        string
	ClientKey         string
	// UseRawCodec 为 true 时 Set/Get 使用零拷贝 rawbytes 编解码（省 proto
	// marshal/unmarshal 两次全量拷贝），要求服务端为带 raw codec 的版本。
	UseRawCodec       bool

	// DirectAddr 非空时启用直连模式：跳过 TiKV 发现/路由/索引，所有操作
	// 直接发往该 gRPC 地址（如 "127.0.0.1:33000"）。
	// DirectRawAddr 可选，指定裸 TCP 数据面地址（sendfile 读用）；
	// 为空时从 DirectAddr 推导（端口 +2）。
	DirectAddr     string
	DirectRawAddr  string
}

func DefaultConfig() *Config {
	return &Config{
		RefreshInterval:  1 * time.Second,
		HeartbeatTimeout: 5 * time.Second,
		UsageThreshold:   0.80,
		RouteCacheSize:   1 * 1024 * 1024 * 1024,
		// 与服务端 32MB 对齐：4MB value 经 proto 包装后 >4MB，
		// 之前默认 4MB 会导致 Get 大 value 全部 ResourceExhausted
		MaxRecvMsgSize:   32 * 1024 * 1024,
	}
}

type Client struct {
	config     *Config
	tikvClient *rawkv.Client
	registry   *InstanceRegistry
	index      *IndexManager
	picker     *InstancePicker
	cache      *RouteCache

	connMu   sync.RWMutex
	conns    map[string]*grpc.ClientConn
	rawConns map[string]*rawConn // GetInto 裸 TCP 数据面连接（端口 = gRPC+2）

	// 直连模式：directInst 是合成的目标实例，所有操作直接发往此地址
	directMode bool
	directInst *InstanceInfo

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	if cfg.MaxRecvMsgSize <= 0 {
		cfg.MaxRecvMsgSize = 16 * 1024 * 1024
	}

	// 直连模式：跳过 TiKV 发现/路由/索引，所有操作直接发往 DirectAddr
	if cfg.DirectAddr != "" {
		return newDirectClient(cfg)
	}

	if cfg.Node == "" {
		return nil, fmt.Errorf("config.Node is required (or set config.DirectAddr for direct mode)")
	}
	if cfg.TiKVPD == "" {
		return nil, fmt.Errorf("config.TiKVPD is required (or set config.DirectAddr for direct mode)")
	}

	ctx, cancel := context.WithCancel(context.Background())

	pdAddrs := strings.Split(cfg.TiKVPD, ",")
	var tlsConfig config.Security
	if cfg.CACert != "" && cfg.ClientCert != "" && cfg.ClientKey != "" {
		tlsConfig.ClusterSSLCA = cfg.CACert
		tlsConfig.ClusterSSLCert = cfg.ClientCert
		tlsConfig.ClusterSSLKey = cfg.ClientKey
	}

	tikvClient, err := rawkv.NewClient(ctx, pdAddrs, tlsConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to TiKV: %v", err)
	}

	if cfg.RouteCacheSize <= 0 {
		cfg.RouteCacheSize = 1 * 1024 * 1024 * 1024
	}
	if cfg.RefreshInterval <= 0 {
		cfg.RefreshInterval = 1 * time.Second
	}
	if cfg.HeartbeatTimeout <= 0 {
		cfg.HeartbeatTimeout = 5 * time.Second
	}
	if cfg.UsageThreshold <= 0 {
		cfg.UsageThreshold = 0.80
	}

	registry := NewInstanceRegistry(tikvClient, cfg.HeartbeatTimeout, cfg.Node)
	index := NewIndexManager(tikvClient)
	picker := NewInstancePicker(registry, cfg.Node, cfg.UsageThreshold)
	cache := NewRouteCache(cfg.RouteCacheSize)

	c := &Client{
		config:     cfg,
		tikvClient: tikvClient,
		registry:   registry,
		index:      index,
		picker:     picker,
		cache:      cache,
		conns:     make(map[string]*grpc.ClientConn),
		rawConns:  make(map[string]*rawConn),
		cancel:     cancel,
	}

	registry.Start(ctx, cfg.RefreshInterval)

	return c, nil
}

// newDirectClient 创建直连模式客户端：不连 TiKV，所有操作直接发往目标实例
func newDirectClient(cfg *Config) (*Client, error) {
	inst := &InstanceInfo{
		Name:    "direct",
		Node:    cfg.Node,
		Addr:    cfg.DirectAddr,
		RawAddr: cfg.DirectRawAddr,
	}

	c := &Client{
		config:     cfg,
		conns:     make(map[string]*grpc.ClientConn),
		rawConns:  make(map[string]*rawConn),
		directMode: true,
		directInst: inst,
	}

	return c, nil
}

func (c *Client) Close() error {
	if !c.directMode {
		c.cancel()
		c.registry.Stop()
		c.tikvClient.Close()
	}

	c.connMu.Lock()
	for _, conn := range c.conns {
		conn.Close()
	}
	c.conns = make(map[string]*grpc.ClientConn)
	for _, rc := range c.rawConns {
		rc.conn.Close()
	}
	c.rawConns = make(map[string]*rawConn)
	c.connMu.Unlock()

	return nil
}

func (c *Client) getConnection(addr string) (*grpc.ClientConn, error) {
	c.connMu.RLock()
	if conn, ok := c.conns[addr]; ok {
		c.connMu.RUnlock()
		return conn, nil
	}
	c.connMu.RUnlock()

	c.connMu.Lock()
	defer c.connMu.Unlock()

	if conn, ok := c.conns[addr]; ok {
		return conn, nil
	}

	// 大 value（如 4MB 数据块）传输：调大 HTTP/2 流控窗口，减少流控往返；
	// 默认 32KB 读写缓冲会把大消息切成小片反复拷贝+syscall，同步调大
	windowSize := int32(16 * 1024 * 1024) // 16MB
	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(c.config.MaxRecvMsgSize)),
		grpc.WithInitialWindowSize(windowSize),
		grpc.WithInitialConnWindowSize(windowSize),
		grpc.WithWriteBufferSize(4*1024*1024),
		grpc.WithReadBufferSize(4*1024*1024),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %v", addr, err)
	}

	c.conns[addr] = conn
	return conn, nil
}

func (c *Client) getKVClient(addr string) (proto.KeyValueServiceClient, error) {
	conn, err := c.getConnection(addr)
	if err != nil {
		return nil, err
	}
	return proto.NewKeyValueServiceClient(conn), nil
}

func (c *Client) Set(ctx context.Context, key string, value []byte) error {
	var inst *InstanceInfo
	if c.directMode {
		inst = c.directInst
	} else {
		var err error
		inst, err = c.picker.Pick()
		if err != nil {
			return err
		}
	}

	kvClient, err := c.getKVClient(inst.Addr)
	if err != nil {
		return err
	}

	req := &proto.SetRequest{
		Key:   []byte(key),
		Value: value,
	}
	if c.config.UseRawCodec {
		// raw codec 对 req.Value 做别名引用：RPC 返回前调用方不得修改 value
		_, err = kvClient.Set(ctx, req, grpc.ForceCodecV2(proto.RawCodecV2))
	} else {
		_, err = kvClient.Set(ctx, req)
	}
	if err != nil {
		return fmt.Errorf("failed to set on instance %s: %v", inst.Name, err)
	}

	if !c.directMode {
		if err := c.index.Put(ctx, key, inst.Name); err != nil {
			log.Printf("WARNING: Failed to update index for key %s: %v", key, err)
		}
		c.cache.Put(key, inst.Name)
	}
	return nil
}

func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	if c.directMode {
		return c.getFromInstanceDirect(ctx, key)
	}

	if instName, ok := c.cache.Get(key); ok {
		if val, err := c.getFromInstance(ctx, instName, key); err == nil {
			return val, nil
		} else if err == ErrKeyNotFound || isInstanceOffline(err) {
			c.cache.Delete(key)
		} else {
			return nil, err
		}
	}

	indexData, err := c.index.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if indexData == nil {
		return nil, ErrKeyNotFound
	}

	c.cache.Put(key, indexData.Instance)

	val, err := c.getFromInstance(ctx, indexData.Instance, key)
	if err != nil {
		if err == ErrKeyNotFound || isInstanceOffline(err) {
			c.cache.Delete(key)
		}
		return nil, err
	}

	return val, nil
}

// getFromInstanceDirect 直连模式读取：跳过 registry/cache/index
func (c *Client) getFromInstanceDirect(ctx context.Context, key string) ([]byte, error) {
	kvClient, err := c.getKVClient(c.directInst.Addr)
	if err != nil {
		return nil, err
	}

	req := &proto.GetRequest{Key: []byte(key)}
	var resp *proto.GetResponse
	if c.config.UseRawCodec {
		resp, err = kvClient.Get(ctx, req, grpc.ForceCodecV2(proto.RawCodecV2))
	} else {
		resp, err = kvClient.Get(ctx, req)
	}
	if err != nil {
		return nil, err
	}
	if !resp.Found {
		return nil, ErrKeyNotFound
	}
	return resp.Value, nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	if c.directMode {
		return c.deleteFromInstanceDirect(ctx, key)
	}

	instName := ""
	if cached, ok := c.cache.Get(key); ok {
		instName = cached
	} else {
		indexData, err := c.index.Get(ctx, key)
		if err != nil {
			return err
		}
		if indexData == nil {
			return nil
		}
		instName = indexData.Instance
	}

	if err := c.deleteFromInstance(ctx, instName, key); err != nil {
		return err
	}

	if err := c.index.Delete(ctx, key); err != nil {
		log.Printf("WARNING: Failed to delete index for key %s: %v", key, err)
	}

	c.cache.Delete(key)
	return nil
}

func (c *Client) deleteFromInstanceDirect(ctx context.Context, key string) error {
	kvClient, err := c.getKVClient(c.directInst.Addr)
	if err != nil {
		return err
	}
	_, err = kvClient.Delete(ctx, &proto.DeleteRequest{Key: []byte(key)})
	return err
}

func (c *Client) MSet(ctx context.Context, kvs map[string][]byte) error {
	if c.directMode {
		kvClient, err := c.getKVClient(c.directInst.Addr)
		if err != nil {
			return err
		}
		protoKVs := make(map[string][]byte, len(kvs))
		for k, v := range kvs {
			protoKVs[k] = v
		}
		_, err = kvClient.MSet(ctx, &proto.MSetRequest{KeyValues: protoKVs})
		return err
	}

	assignments := make(map[string]map[string][]byte)

	for key, value := range kvs {
		inst, err := c.picker.Pick()
		if err != nil {
			return err
		}
		if assignments[inst.Addr] == nil {
			assignments[inst.Addr] = make(map[string][]byte)
		}
		assignments[inst.Addr][key] = value
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(assignments))

	instMap := make(map[string]*InstanceInfo)
	for _, inst := range c.registry.GetActiveInstances() {
		instMap[inst.Addr] = inst
	}

	for addr, batch := range assignments {
		wg.Add(1)
		go func(addr string, batch map[string][]byte) {
			defer wg.Done()

			kvClient, err := c.getKVClient(addr)
			if err != nil {
				errCh <- err
				return
			}

			protoKVs := make(map[string][]byte)
			for k, v := range batch {
				protoKVs[k] = v
			}

			_, err = kvClient.MSet(ctx, &proto.MSetRequest{
				KeyValues: protoKVs,
			})
			if err != nil {
				errCh <- fmt.Errorf("failed to mset on %s: %v", addr, err)
				return
			}

			inst := instMap[addr]
			if inst == nil {
				return
			}
			indexKVs := make(map[string]string)
			for k := range batch {
				indexKVs[k] = inst.Name
				c.cache.Put(k, inst.Name)
			}
			c.index.BatchPut(ctx, indexKVs)
		}(addr, batch)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	if c.directMode {
		return c.mgetFromInstanceDirect(ctx, keys)
	}

	assignments := make(map[string][]string)
	keyToInst := make(map[string]string)

	cacheHits := make(map[string]bool)
	for _, key := range keys {
		if instName, ok := c.cache.Get(key); ok {
			assignments[instName] = append(assignments[instName], key)
			keyToInst[key] = instName
			cacheHits[key] = true
		}
	}

	var cacheMisses []string
	for _, key := range keys {
		if !cacheHits[key] {
			cacheMisses = append(cacheMisses, key)
		}
	}

	if len(cacheMisses) > 0 {
		indexData, _ := c.index.BatchGet(ctx, cacheMisses)
		for key, data := range indexData {
			assignments[data.Instance] = append(assignments[data.Instance], key)
			keyToInst[key] = data.Instance
			c.cache.Put(key, data.Instance)
		}
	}

	results := make(map[string][]byte)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var errs []string

	for instName, batchKeys := range assignments {
		wg.Add(1)
		go func(instName string, batchKeys []string) {
			defer wg.Done()

			val, err := c.mgetFromInstance(ctx, instName, batchKeys)
			if err != nil {
				// 不再静默吞错：实例失败必须让调用方感知，
				// 否则"实例挂了"和"key 不存在"无法区分
				mu.Lock()
				errs = append(errs, fmt.Sprintf("instance %s: %v", instName, err))
				mu.Unlock()
				return
			}

			mu.Lock()
			for k, v := range val {
				results[k] = v
			}
			mu.Unlock()
		}(instName, batchKeys)
	}

	wg.Wait()
	if len(errs) > 0 {
		// 返回部分结果 + 错误，由调用方决定是否可用
		return results, fmt.Errorf("mget partial failure: %s", strings.Join(errs, "; "))
	}
	return results, nil
}

func (c *Client) mgetFromInstanceDirect(ctx context.Context, keys []string) (map[string][]byte, error) {
	kvClient, err := c.getKVClient(c.directInst.Addr)
	if err != nil {
		return nil, err
	}
	protoKeys := make([][]byte, len(keys))
	for i, k := range keys {
		protoKeys[i] = []byte(k)
	}
	resp, err := kvClient.MGet(ctx, &proto.MGetRequest{Keys: protoKeys})
	if err != nil {
		return nil, err
	}
	results := make(map[string][]byte)
	for k, v := range resp.KeyValues {
		results[k] = v
	}
	return results, nil
}

func (c *Client) MDelete(ctx context.Context, keys []string) error {
	if c.directMode {
		kvClient, err := c.getKVClient(c.directInst.Addr)
		if err != nil {
			return err
		}
		protoKeys := make([][]byte, len(keys))
		for i, k := range keys {
			protoKeys[i] = []byte(k)
		}
		_, err = kvClient.MDelete(ctx, &proto.MDeleteRequest{Keys: protoKeys})
		return err
	}

	assignments := make(map[string][]string)

	for _, key := range keys {
		instName := ""
		if cached, ok := c.cache.Get(key); ok {
			instName = cached
		} else {
			indexData, _ := c.index.Get(ctx, key)
			if indexData != nil {
				instName = indexData.Instance
			}
		}
		if instName != "" {
			assignments[instName] = append(assignments[instName], key)
		}
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(assignments))

	for instName, batchKeys := range assignments {
		wg.Add(1)
		go func(instName string, batchKeys []string) {
			defer wg.Done()
			if err := c.mdeleteFromInstance(ctx, instName, batchKeys); err != nil {
				errCh <- err
			}
		}(instName, batchKeys)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}

	// 索引批量删除（底层一次 rawkv BatchDelete），route cache 逐个清
	c.index.BatchDelete(ctx, keys)
	for _, key := range keys {
		c.cache.Delete(key)
	}

	return nil
}

func (c *Client) getFromInstance(ctx context.Context, instName, key string) ([]byte, error) {
	insts := c.registry.GetActiveInstances()
	inst, ok := insts[instName]
	if !ok {
		return nil, fmt.Errorf("instance %s is offline", instName)
	}

	kvClient, err := c.getKVClient(inst.Addr)
	if err != nil {
		return nil, err
	}

	req := &proto.GetRequest{
		Key: []byte(key),
	}
	var resp *proto.GetResponse
	if c.config.UseRawCodec {
		resp, err = kvClient.Get(ctx, req, grpc.ForceCodecV2(proto.RawCodecV2))
	} else {
		resp, err = kvClient.Get(ctx, req)
	}
	if err != nil {
		return nil, err
	}

	if !resp.Found {
		return nil, ErrKeyNotFound
	}

	return resp.Value, nil
}

func (c *Client) deleteFromInstance(ctx context.Context, instName, key string) error {
	insts := c.registry.GetActiveInstances()
	inst, ok := insts[instName]
	if !ok {
		return fmt.Errorf("instance %s is offline", instName)
	}

	kvClient, err := c.getKVClient(inst.Addr)
	if err != nil {
		return err
	}

	_, err = kvClient.Delete(ctx, &proto.DeleteRequest{
		Key: []byte(key),
	})
	return err
}

func (c *Client) mgetFromInstance(ctx context.Context, instName string, keys []string) (map[string][]byte, error) {
	insts := c.registry.GetActiveInstances()
	inst, ok := insts[instName]
	if !ok {
		return nil, fmt.Errorf("instance %s is offline", instName)
	}

	kvClient, err := c.getKVClient(inst.Addr)
	if err != nil {
		return nil, err
	}

	protoKeys := make([][]byte, len(keys))
	for i, k := range keys {
		protoKeys[i] = []byte(k)
	}

	resp, err := kvClient.MGet(ctx, &proto.MGetRequest{
		Keys: protoKeys,
	})
	if err != nil {
		return nil, err
	}

	results := make(map[string][]byte)
	for k, v := range resp.KeyValues {
		results[k] = v
	}

	return results, nil
}

func (c *Client) mdeleteFromInstance(ctx context.Context, instName string, keys []string) error {
	insts := c.registry.GetActiveInstances()
	inst, ok := insts[instName]
	if !ok {
		return fmt.Errorf("instance %s is offline", instName)
	}

	kvClient, err := c.getKVClient(inst.Addr)
	if err != nil {
		return err
	}

	protoKeys := make([][]byte, len(keys))
	for i, k := range keys {
		protoKeys[i] = []byte(k)
	}

	_, err = kvClient.MDelete(ctx, &proto.MDeleteRequest{
		Keys: protoKeys,
	})
	return err
}

func isInstanceOffline(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) ||
		(err.Error() != "" && (contains(err.Error(), "connection") || contains(err.Error(), "offline")))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
