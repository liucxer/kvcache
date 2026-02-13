package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"

	"kvcache/api"
	"kvcache/config"
	"kvcache/service"
	"kvcache/storage"
)

const (
	// 端口范围
	minPort = 33000
	maxPort = 33100
)

func main() {
	// 初始化配置
	cfg := config.DefaultConfig()

	// 创建存储实例
	store, err := storage.NewStorage(cfg)
	if err != nil {
		log.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Stop()

	// 创建业务逻辑服务
	kvService := service.NewKVService(store, cfg)

	// 设置监控指标处理
	http.Handle("/metrics", promhttp.Handler())

	// 自动检测可用端口
	grpcPort, httpPort := findAvailablePorts()
	log.Printf("Selected ports: GRPC=%d, HTTP=%d", grpcPort, httpPort)

	// 启动gRPC服务器
	grpcAddr := fmt.Sprintf(":%d", grpcPort)
	grpcServer := startGRPCServer(grpcAddr, kvService)

	// 启动HTTP服务器
	httpAddr := fmt.Sprintf(":%d", httpPort)
	httpServer := startHTTPServer(httpAddr, kvService)

	// 等待中断信号
	waitForShutdown(grpcServer, httpServer)
}

// findAvailablePorts 查找可用的端口对
func findAvailablePorts() (int, int) {
	// 确保从偶数端口开始
	startPort := minPort
	if startPort%2 != 0 {
		startPort++
	}

	for port := startPort; port <= maxPort-1; port += 2 {
		// 检查GRPC端口是否可用（偶数）
		grpcAddr := fmt.Sprintf(":%d", port)
		grpcLis, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			continue
		}
		grpcLis.Close()

		// 检查HTTP端口是否可用（奇数）
		httpAddr := fmt.Sprintf(":%d", port+1)
		httpLis, err := net.Listen("tcp", httpAddr)
		if err != nil {
			continue
		}
		httpLis.Close()

		// 找到可用端口对
		return port, port + 1
	}

	// 没有找到可用端口
	log.Fatalf("No available ports found in range %d-%d", minPort, maxPort)
	return 0, 0
}

// startGRPCServer 启动gRPC服务器
func startGRPCServer(addr string, service *service.KVService) *grpc.Server {
	// 创建gRPC服务器
	server := grpc.NewServer()

	// 创建gRPC服务实例
	grpcService := api.NewGRPCServer(service)

	// 注册服务
	grpcService.Register(server)

	// 启动服务器
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

// startHTTPServer 启动HTTP服务器
func startHTTPServer(addr string, service *service.KVService) *http.Server {
	// 创建HTTP服务实例
	httpService := api.NewHTTPServer(service)

	// 创建HTTP服务器
	server := &http.Server{
		Addr:    addr,
		Handler: nil, // 使用gin默认路由
	}

	// 启动服务器
	go func() {
		log.Printf("HTTP server started on %s", addr)
		if err := httpService.Run(addr); err != nil {
			log.Fatalf("Failed to serve HTTP: %v", err)
		}
	}()

	return server
}

// waitForShutdown 等待中断信号并优雅关闭服务器
func waitForShutdown(grpcServer *grpc.Server, httpServer *http.Server) {
	// 创建通道接收中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 等待中断信号
	<-quit
	log.Println("Shutting down servers...")

	// 创建上下文用于优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 关闭gRPC服务器
	grpcServer.GracefulStop()
	log.Println("gRPC server stopped")

	// 关闭HTTP服务器
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	} else {
		log.Println("HTTP server stopped")
	}

	log.Println("All servers stopped gracefully")
}
