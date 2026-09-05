// nefsbench: 旧 HTTP vs 新 TCP 完整对比
package main

import (
	"context"
	"fmt"
	"time"

	"kvcache/nefsagent"
	"kvcache/nefsproxy"
)

func main() {
	ctx := context.Background()

	fmt.Println("========== 旧 HTTP 模式 (proxy.py :9527) ==========")
	oldC := nefsproxy.NewClient("http://100.71.128.12:9527", "95279527", nefsproxy.WithTimeout(10*time.Second))

	fmt.Println("\n--- echo ok 串行 50 ---")
	var oldEcho []time.Duration
	for i := 0; i < 50; i++ {
		t0 := time.Now()
		oldC.ExecOut(ctx, "echo ok")
		oldEcho = append(oldEcho, time.Since(t0))
	}
	printStats("HTTP-echo", oldEcho)

	fmt.Println("\n--- 并发 10×50 echo ---")
	start := time.Now()
	done := make(chan struct{})
	for w := 0; w < 10; w++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for i := 0; i < 50; i++ {
				oldC.ExecOut(ctx, "echo ok")
			}
		}()
	}
	for w := 0; w < 10; w++ {
		<-done
	}
	oldTotal := time.Since(start)
	oldQPS := float64(500) / oldTotal.Seconds()
	fmt.Printf("  500 请求总耗时: %v, QPS: %.0f\n", oldTotal, oldQPS)

	fmt.Println("\n========== 新 TCP 长连接模式 (nefsagent :9528) ==========")
	newC, err := nefsagent.NewClient("100.71.128.12:9528", "nefsagent", nefsagent.WithPoolSize(4))
	if err != nil {
		fmt.Println("connect err:", err)
		return
	}
	defer newC.Close()
	newC.Health(ctx)

	fmt.Println("\n--- echo ok 串行 50 ---")
	var newEcho []time.Duration
	for i := 0; i < 50; i++ {
		t0 := time.Now()
		newC.ExecOut(ctx, "echo ok")
		newEcho = append(newEcho, time.Since(t0))
	}
	printStats("TCP-echo", newEcho)

	fmt.Println("\n--- 并发 10×50 echo ---")
	start = time.Now()
	for w := 0; w < 10; w++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for i := 0; i < 50; i++ {
				newC.ExecOut(ctx, "echo ok")
			}
		}()
	}
	for w := 0; w < 10; w++ {
		<-done
	}
	newTotal := time.Since(start)
	newQPS := float64(500) / newTotal.Seconds()
	fmt.Printf("  500 请求总耗时: %v, QPS: %.0f\n", newTotal, newQPS)

	fmt.Println("\n========== 对比总结 ==========")
	fmt.Printf("  %-25s %10s %10s %10s\n", "指标", "旧 HTTP", "新 TCP", "加速")
	fmt.Printf("  %-25s %9.1fms %9.1fms %9.1fx\n", "echo 串行 avg", favg(oldEcho), favg(newEcho), favg(oldEcho)/favg(newEcho))
	fmt.Printf("  %-25s %9.0f QPS %9.0f QPS %9.1fx\n", "并发 500 QPS", oldQPS, newQPS, newQPS/oldQPS)
	fmt.Printf("\n  网络 RTT (ICMP): ~5ms\n")
	fmt.Printf("  新协议 echo avg: %.1fms (网络+协议+sh执行)\n", favg(newEcho))
}

func favg(ds []time.Duration) float64 {
	var sum time.Duration
	for _, d := range ds {
		sum += d
	}
	return float64(sum) / float64(len(ds)) / float64(time.Millisecond)
}

func printStats(name string, ds []time.Duration) {
	var sum time.Duration
	min, max := ds[0], ds[0]
	for _, d := range ds {
		sum += d
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
	}
	fmt.Printf("  %-15s n=%2d min=%5.1fms avg=%5.1fms max=%6.1fms\n",
		name, len(ds),
		float64(min)/float64(time.Millisecond),
		float64(sum)/float64(len(ds))/float64(time.Millisecond),
		float64(max)/float64(time.Millisecond))
}
