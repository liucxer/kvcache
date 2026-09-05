package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tikv/client-go/v2/config"
	"github.com/tikv/client-go/v2/rawkv"
	"google.golang.org/grpc"

	"kvcache/api"
	appconfig "kvcache/config"
	"kvcache/service"
	"kvcache/storage"
)

const (
	minPort           = 33000
	maxPort           = 33100
	instanceKeyPrefix = "/kvcache/instances/"
	heartbeatInterval = 1 * time.Second
)

var (
	instanceName     = flag.String("name", "", "Instance name (required)")
	nodeName         = flag.String("node", "", "Node name (required)")
	grpcAddrFlag     = flag.String("grpc-addr", "", "gRPC address")
	tikvPD           = flag.String("tikv-pd", "", "TiKV PD addresses (comma-separated)")
	caPath           = flag.String("ca-path", "", "TLS CA certificate path")
	certPath         = flag.String("cert-path", "", "TLS certificate path")
	keyPath          = flag.String("key-path", "", "TLS key path")
	dataDir          = flag.String("data-dir", "", "RocksDB data directory (default ./data)")
	valueDir         = flag.String("value-dir", "", "Large value disk store directory (default ./value_data)")
	rawAddrFlag      = flag.String("raw-addr", "", "Raw data plane (sendfile read) address (default: gRPC port + 2)")
	rawWriteAddrFlag = flag.String("raw-write-addr", "", "Raw write data plane (pipelined TCP Set) address (default: raw addr + 1)")
	readaheadKB      = flag.Int("readahead-kb", 4096, "read_ahead_kb for the value-dir block device (0 = disable tuning)")
)

func main() {
	flag.Parse()

	// 使用所有 CPU 核心
	numCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPU)
	log.Printf("GOMAXPROCS set to %d (NumCPU=%d)", numCPU, numCPU)

	if *instanceName == "" || *nodeName == "" || *tikvPD == "" {
		log.Fatalf("--name, --node, and --tikv-pd are required")
	}

	// Parse PD addresses (comma-separated)
	pdAddrs := strings.Split(*tikvPD, ",")
	for i := range pdAddrs {
		pdAddrs[i] = strings.TrimSpace(pdAddrs[i])
	}

	cfg := appconfig.DefaultConfig()

	if *dataDir != "" {
		cfg.RocksDB.Path = *dataDir
	}
	if *valueDir != "" {
		cfg.Value.DiskPath = *valueDir
	}

	// value 大文件顺序读的性能关键项：调大所在块设备的 readahead 窗口。
	// 只调 value-dir；RocksDB 目录以随机小 IO 为主，不动。
	if *readaheadKB > 0 {
		setReadaheadForPath(cfg.Value.DiskPath, *readaheadKB)
	}

	store, err := storage.NewStorage(cfg)
	if err != nil {
		log.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	kvService := service.NewKVService(store, cfg)

	http.Handle("/metrics", promhttp.Handler())

	var grpcAddr, httpAddr string
	if *grpcAddrFlag != "" {
		grpcAddr = *grpcAddrFlag
		httpAddr = deriveHTTPAddr(grpcAddr)
	} else {
		grpcAddr, httpAddr = findAvailablePorts()
	}
	log.Printf("Selected ports: GRPC=%s, HTTP=%s", grpcAddr, httpAddr)

	// 裸 TCP 数据面（GetInto/sendfile 读）：--raw-addr 指定，缺省 gRPC 端口 +2。
	// 先于注册启动：启动失败则注册空 raw_addr，客户端自动回退 gRPC 路径。
	rawAddr := *rawAddrFlag
	if rawAddr == "" {
		rawAddr = deriveOffsetAddr(grpcAddr, 2)
	}
	advertisedRawAddr := rawAddr
	if rawSrv, err := api.StartRawDataServer(rawAddr, store); err != nil {
		log.Printf("WARNING: raw data server on %s disabled: %v", rawAddr, err)
		advertisedRawAddr = ""
	} else {
		defer rawSrv.Close()
	}

	// 裸 TCP 写数据面（流水线 Set）：--raw-write-addr 指定，缺省 raw 地址 +1。
	// 独立端口，与读数据面/注册无关；失败仅告警不影响主服务。
	rawWriteAddr := *rawWriteAddrFlag
	if rawWriteAddr == "" {
		rawWriteAddr = deriveOffsetAddr(rawAddr, 1)
	}
	if rawWrSrv, err := api.StartRawWriteServer(rawWriteAddr, kvService); err != nil {
		log.Printf("WARNING: raw write server on %s disabled: %v", rawWriteAddr, err)
	} else {
		defer rawWrSrv.Close()
	}

	// Configure TiKV client with TLS if certificates provided
	sec := config.Security{}
	if *caPath != "" && *certPath != "" && *keyPath != "" {
		sec.ClusterSSLCA = *caPath
		sec.ClusterSSLCert = *certPath
		sec.ClusterSSLKey = *keyPath
		log.Println("TiKV TLS enabled")
	}

	tikvClient, err := rawkv.NewClient(context.Background(), pdAddrs, sec)
	if err != nil {
		log.Fatalf("Failed to connect to TiKV: %v", err)
	}
	defer tikvClient.Close()

	registrationCtx, cancelReg := context.WithCancel(context.Background())
	defer cancelReg()

	if err := registerInstance(registrationCtx, tikvClient, *instanceName, *nodeName, grpcAddr, advertisedRawAddr, store); err != nil {
		log.Fatalf("Failed to register instance: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runHeartbeat(registrationCtx, tikvClient, *instanceName, *nodeName, grpcAddr, advertisedRawAddr, store)
	}()

	grpcServer := startGRPCServer(grpcAddr, kvService)
	httpServer := startHTTPServer(httpAddr, kvService)

	waitForShutdown(grpcServer, httpServer, cancelReg, &wg, tikvClient, *instanceName)
}

type instanceInfo struct {
	Name      string `json:"name"`
	Node      string `json:"node"`
	Addr      string `json:"addr"`
	RawAddr   string `json:"raw_addr,omitempty"`
	Capacity  int64  `json:"capacity"`
	Available int64  `json:"available"`
	Used      int64  `json:"used"`
	StartTime int64  `json:"start_time"`
}

func buildInstanceInfo(name, node, addr, rawAddr string, store storage.Storage) *instanceInfo {
	capacity, available, used, err := store.GetDiskCapacity()
	if err != nil {
		log.Printf("WARNING: Failed to get disk capacity: %v", err)
		capacity, available, used = 0, 0, 0
	}

	return &instanceInfo{
		Name:      name,
		Node:      node,
		Addr:      addr,
		RawAddr:   rawAddr,
		Capacity:  capacity,
		Available: available,
		Used:      used,
		StartTime: time.Now().Unix(),
	}
}

func registerInstance(ctx context.Context, tikvClient *rawkv.Client, name, node, addr, rawAddr string, store storage.Storage) error {
	info := buildInstanceInfo(name, node, addr, rawAddr, store)

	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal instance info: %v", err)
	}

	key := []byte(instanceKeyPrefix + name)
	if err := tikvClient.Put(ctx, key, data); err != nil {
		return fmt.Errorf("failed to register instance: %v", err)
	}

	log.Printf("Registered instance %s at node %s, addr %s", name, node, addr)
	return nil
}

func unregisterInstance(ctx context.Context, tikvClient *rawkv.Client, name string) error {
	key := []byte(instanceKeyPrefix + name)
	if err := tikvClient.Delete(ctx, key); err != nil {
		return fmt.Errorf("failed to unregister instance: %v", err)
	}
	log.Printf("Unregistered instance %s", name)
	return nil
}

func runHeartbeat(ctx context.Context, tikvClient *rawkv.Client, name, node, addr, rawAddr string, store storage.Storage) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			info := buildInstanceInfo(name, node, addr, rawAddr, store)
			data, err := json.Marshal(info)
			if err != nil {
				log.Printf("WARNING: Failed to marshal heartbeat: %v", err)
				continue
			}

			key := []byte(instanceKeyPrefix + name)
			if err := tikvClient.Put(ctx, key, data); err != nil {
				log.Printf("WARNING: Failed to send heartbeat: %v", err)
			}
		case <-ctx.Done():
			log.Printf("Heartbeat goroutine stopped")
			return
		}
	}
}

func findAvailablePorts() (string, string) {
	startPort := minPort
	if startPort%2 != 0 {
		startPort++
	}

	for port := startPort; port <= maxPort-1; port += 2 {
		grpcAddr := fmt.Sprintf(":%d", port)
		grpcLis, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			continue
		}
		grpcLis.Close()

		httpAddr := fmt.Sprintf(":%d", port+1)
		httpLis, err := net.Listen("tcp", httpAddr)
		if err != nil {
			continue
		}
		httpLis.Close()

		return grpcAddr, httpAddr
	}

	log.Fatalf("No available ports found in range %d-%d", minPort, maxPort)
	return "", ""
}

func deriveHTTPAddr(grpcAddr string) string {
	httpAddr := deriveOffsetAddr(grpcAddr, 1)
	lis, err := net.Listen("tcp", httpAddr)
	if err != nil {
		log.Fatalf("HTTP port %s not available: %v", httpAddr, err)
	}
	lis.Close()
	return httpAddr
}

// deriveOffsetAddr 由 gRPC 地址推导同主机偏移端口的地址（HTTP=+1，裸数据面=+2）
func deriveOffsetAddr(grpcAddr string, offset int) string {
	parts := strings.Split(grpcAddr, ":")
	if len(parts) != 2 {
		log.Fatalf("Invalid gRPC address format: %s", grpcAddr)
	}

	port := 0
	fmt.Sscanf(parts[1], "%d", &port)
	return fmt.Sprintf(":%d", port+offset)
}

func startGRPCServer(addr string, svc *service.KVService) *grpc.Server {
	maxMsgSize := 32 * 1024 * 1024 // 32MB
	// 大 value（如 4MB 数据块）场景下，HTTP/2 默认 64KB 流控窗口需要多次往返，
	// 调大窗口对大消息吞吐影响显著
	windowSize := int32(16 * 1024 * 1024) // 16MB
	numWorkers := runtime.NumCPU()
	if numWorkers < 16 {
		numWorkers = 16
	}
	if numWorkers > 128 {
		numWorkers = 128
	}
	server := grpc.NewServer(
		grpc.MaxRecvMsgSize(maxMsgSize),
		grpc.MaxSendMsgSize(maxMsgSize),
		grpc.MaxConcurrentStreams(uint32(numWorkers*8)),
		grpc.NumStreamWorkers(uint32(numWorkers)),
		grpc.InitialWindowSize(windowSize),
		grpc.InitialConnWindowSize(windowSize),
		// 默认 32KB 读写缓冲会把 4MB 消息切成上百个小片，每片一次拷贝+syscall，
		// 调大到 4MB 减少传输层开销（pprof 实测 bufio/bufWriter 占比显著）
		grpc.WriteBufferSize(4*1024*1024),
		grpc.ReadBufferSize(4*1024*1024),
	)
	log.Printf("gRPC server configured with %d stream workers, %d max concurrent streams", numWorkers, numWorkers*8)
	grpcService := api.NewGRPCServer(svc)
	grpcService.Register(server)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	go func() {
		log.Printf("gRPC server started on %s", addr)
		if err := server.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	return server
}

func startHTTPServer(addr string, svc *service.KVService) *http.Server {
	httpService := api.NewHTTPServer(svc)

	server := &http.Server{
		Addr:    addr,
		Handler: nil,
	}

	go func() {
		log.Printf("HTTP server started on %s", addr)
		if err := httpService.Run(addr); err != nil {
			log.Fatalf("Failed to serve HTTP: %v", err)
		}
	}()

	return server
}

func waitForShutdown(grpcServer *grpc.Server, httpServer *http.Server, cancelReg context.CancelFunc, wg *sync.WaitGroup, tikvClient *rawkv.Client, name string) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down servers...")

	grpcServer.GracefulStop()
	log.Println("gRPC server stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	} else {
		log.Println("HTTP server stopped")
	}

	cancelReg()
	wg.Wait()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := unregisterInstance(shutdownCtx, tikvClient, name); err != nil {
		log.Printf("Failed to unregister instance: %v", err)
	}

	log.Println("All servers stopped gracefully")
}
