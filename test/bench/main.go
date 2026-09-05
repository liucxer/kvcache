package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof" // SDK 侧 pprof，监听 :16060
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"kvcache/client"
	"kvcache/proto"
)

var (
	workers          = flag.Int("workers", 32, "concurrent workers")
	duration         = flag.Duration("duration", 60*time.Second, "test duration")
	valueSize        = flag.Int("value-size", 4*1024*1024, "value size in bytes")
	mode             = flag.String("mode", "write", "write|read")
	prefix           = flag.String("prefix", "bench60", "key prefix")
	startSeq         = flag.Int("start-seq", 0, "write mode: starting seq number")
	countPerWorker   = flag.Int("count-per-worker", 0, "write mode: keys per worker; 0 = loop until duration elapses")
	readMaxSeq       = flag.Int("read-max-seq", 1200, "read mode: random seq in [0, N), keys <prefix>:w<wid>:<seq>")
	readWidMod       = flag.Int("read-wid-mod", 64, "read mode: worker id modulo (data was written by this many workers)")
	clientPerWorker  = flag.Bool("client-per-worker", true, "each worker uses an independent SDK client")
	useStream        = flag.Bool("stream", false, "use streaming SetStream/GetStream (1MB chunks) instead of unary Set/Get")
	useRaw           = flag.Bool("raw", false, "unary Set/Get with zero-copy rawbytes codec (skips proto marshal/unmarshal)")
	useInto          = flag.Bool("into", false, "read mode: GetInto over raw TCP data plane (sendfile server-side, caller-owned buffer)")
	directAddrs      = flag.String("direct", "", "bypass SDK routing/index: comma-separated instance gRPC addrs, raw RPC round-robin; with -raw-write these are the raw-write TCP data plane addrs")
	sdkDirectAddr    = flag.String("direct-addr", "", "SDK direct mode: single instance gRPC addr, bypass TiKV discovery/routing/index; enables --into without TiKV")
	sdkDirectRawAddr = flag.String("direct-raw-addr", "", "SDK direct mode: raw TCP data plane addr for --into (sendfile); defaults to gRPC port+2")
	useRawWrite      = flag.Bool("raw-write", false, "write mode over raw TCP data plane with sendmsg/sendmmsg (direct addrs must be raw-write ports)")
	batchSize        = flag.Int("batch", 8, "raw-write: frames per sendmmsg batch (1 = single-frame sendmsg)")
	node             = flag.String("node", "146", "local node name")
	tikvPD           = flag.String("tikv-pd", "10.153.28.202:12379,10.153.28.203:12379,10.153.28.204:12379", "TiKV PD addrs")
	caPath           = flag.String("ca", "/nefsdata/meta/tikv-deploy/pd-12379/tls/ca.crt", "TiKV TLS CA")
	certPath         = flag.String("cert", "/nefsdata/meta/tikv-deploy/pd-12379/tls/pd.crt", "TiKV TLS cert")
	keyPath          = flag.String("key", "/nefsdata/meta/tikv-deploy/pd-12379/tls/pd.pem", "TiKV TLS key")
)

// direct 模式的流式写入：与 client.SetStream 相同的分帧逻辑，不走路由/索引
func directSetStream(ctx context.Context, cli proto.KVStreamServiceClient, key string, value []byte) error {
	stream, err := cli.SetStream(ctx)
	if err != nil {
		return err
	}
	for off := 0; ; off += proto.StreamChunkSize {
		end := off + proto.StreamChunkSize
		if end > len(value) {
			end = len(value)
		}
		chunk := &proto.SetChunk{Data: value[off:end]}
		if off == 0 {
			chunk.Key = []byte(key)
		}
		if err := stream.Send(chunk); err != nil {
			return err
		}
		if end == len(value) {
			break
		}
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("set stream rejected: %s", resp.Error)
	}
	return nil
}

// rawWriteConn 裸 TCP 写数据面客户端：sendmsg/sendmmsg 批量发送 + 流水线 ACK。
// 协议（与服务端 api/raw_write.go 对称）：
//
//	请求: [4B keyLen][key][8B valueLen][value]；响应: [1B status][8B bodyLen]
type rawWriteConn struct {
	c  *net.TCPConn
	fd int
}

func dialRawWrite(addr string) (*rawWriteConn, error) {
	c, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	tc, ok := c.(*net.TCPConn)
	if !ok {
		c.Close()
		return nil, fmt.Errorf("not tcp: %s", addr)
	}
	tc.SetNoDelay(true)
	tc.SetWriteBuffer(128 << 20) // 大 sndbuf：batch 内一次塞入多帧少触发 EAGAIN
	tc.SetReadBuffer(1 << 20)

	fd := -1
	sc, err := tc.SyscallConn()
	if err != nil {
		tc.Close()
		return nil, err
	}
	sc.Control(func(f uintptr) { fd = int(f) })
	if fd < 0 {
		tc.Close()
		return nil, fmt.Errorf("raw write: no fd for %s", addr)
	}
	return &rawWriteConn{c: tc, fd: fd}, nil
}

func (rw *rawWriteConn) Close() error { return rw.c.Close() }

func (rw *rawWriteConn) waitWritable() error {
	for {
		_, err := unix.Poll([]unix.PollFd{{Fd: int32(rw.fd), Events: unix.POLLOUT}}, -1)
		if err == unix.EINTR {
			continue
		}
		return err
	}
}

// setBatch 一次性系统调用发送 batch 帧并逐帧读取 ACK。
// TCP 是字节流无消息边界，批量写用一次 writev（= write 语义的 sendmmsg 等价物；
// 内核一次调用拷贝全部段，区别仅少建 header）即可达到"批量发送替代逐包 write"的目的。
// 各帧 value 可共享同一切片：调用返回即完成内核拷贝，调用方随后即可改写。
func (rw *rawWriteConn) setBatch(frames []rawFrame) error {
	bufs := make([][]byte, 0, 3*len(frames))
	for _, f := range frames {
		hdr := make([]byte, 12)
		binary.BigEndian.PutUint32(hdr[0:4], uint32(len(f.key)))
		binary.BigEndian.PutUint64(hdr[4:12], uint64(len(f.value)))
		bufs = append(bufs, hdr, []byte(f.key), f.value)
	}

	pos, off := 0, 0
	for pos < len(bufs) {
		cur := make([][]byte, 0, len(bufs)-pos)
		for i := pos; i < len(bufs); i++ {
			if i == pos {
				cur = append(cur, bufs[i][off:])
			} else {
				cur = append(cur, bufs[i])
			}
		}
		n, err := unix.Writev(rw.fd, cur)
		if err == unix.EAGAIN || err == unix.EINTR {
			if werr := rw.waitWritable(); werr != nil {
				return fmt.Errorf("raw write wait: %v", werr)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("raw write writev: %v", err)
		}
		if n <= 0 {
			return fmt.Errorf("raw write writev short: n=%d", n)
		}
		for n > 0 && pos < len(bufs) {
			if left := len(bufs[pos]) - off; n >= left {
				n -= left
				pos++
				off = 0
			} else {
				off += n
				n = 0
			}
		}
	}

	ack := make([]byte, 9)
	for range frames {
		if _, err := io.ReadFull(rw.c, ack); err != nil {
			return fmt.Errorf("raw write ack: %v", err)
		}
		if ack[0] != 0 {
			return fmt.Errorf("raw write ack status=%d", ack[0])
		}
	}
	return nil
}

// rawFrame 单个写帧（key 独立，value 可共享）
type rawFrame struct {
	key   string
	value []byte
}

func directGetStream(ctx context.Context, cli proto.KVStreamServiceClient, key string) ([]byte, error) {
	stream, err := cli.GetStream(ctx, &proto.GetRequest{Key: []byte(key)})
	if err != nil {
		return nil, err
	}
	var buf []byte
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return buf, nil
		}
		if err != nil {
			return nil, err
		}
		buf = append(buf, chunk.Data...)
	}
}

func main() {
	flag.Parse()
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	// SDK 侧 pprof：go tool pprof http://<host>:16060/debug/pprof/profile
	// （6060 被 146 上的 nginx 占用，用 16060）
	go func() {
		log.Println("pprof listening on :16060")
		if err := http.ListenAndServe(":16060", nil); err != nil {
			log.Printf("pprof server error: %v", err)
		}
	}()

	fmt.Println("=== kvcache SDK benchmark ===")
	fmt.Printf("mode=%s workers=%d duration=%v valueSize=%d clientPerWorker=%v stream=%v raw=%v into=%v direct=%q directAddr=%q directRawAddr=%q rawWrite=%v batch=%d\n",
		*mode, *workers, *duration, *valueSize, *clientPerWorker, *useStream, *useRaw, *useInto, *directAddrs, *sdkDirectAddr, *sdkDirectRawAddr, *useRawWrite, *batchSize)
	if *mode == "read" {
		readGB := float64(*workers) * float64(*readMaxSeq) * float64(*valueSize) / 1024 / 1024 / 1024
		fmt.Printf("read mode: exact traversal, %d keys/worker over %d writers => total read %.1f GB (<= written), each key read exactly once\n",
			*readMaxSeq, *readWidMod, readGB)
	}

	ctx := context.Background()

	// setFn/getFn：统一的操作入口，direct 模式绕开 SDK 路由与 TiKV 索引
	var setFn func(ctx context.Context, wid int, key string, value []byte) error
	var getFn func(ctx context.Context, wid int, key string) ([]byte, error)
	var clients []*client.Client // SDK 模式持有，GetInto 走这里

	if *useInto && *directAddrs != "" {
		log.Fatal("-into requires SDK mode, cannot combine with -direct (bench-level direct); use -direct-addr for SDK direct mode with -into")
	}
	if *useRawWrite && *directAddrs == "" {
		log.Fatal("-raw-write requires -direct with the instance raw-write TCP addrs")
	}
	if *useRawWrite && *mode == "read" {
		log.Fatal("-raw-write is write-mode only")
	}
	if *batchSize < 1 {
		log.Fatalf("-batch must be >= 1, got %d", *batchSize)
	}
	if *mode == "read" {
		// 精确遍历读约束：读取量 ≤ 写入量，且每个 key 恰好读一次
		if *readMaxSeq <= 0 {
			log.Fatalf("read mode: --read-max-seq(%d) must be > 0 and == write --count-per-worker", *readMaxSeq)
		}
		if *workers > *readWidMod {
			log.Fatalf("read mode: --workers(%d) must be <= --read-wid-mod(%d): read workers cannot exceed write workers, otherwise keys are re-read", *workers, *readWidMod)
		}
	}

	// rwConns：raw-write 模式的裸 TCP 写连接池（--direct 给出的 raw-write 端口）
	var rwConns []*rawWriteConn

	if *directAddrs != "" {
		addrs := strings.Split(*directAddrs, ",")
		if *useRawWrite {
			// raw-write 数据面：每 worker 一条连接，按 worker id 固定取模（保持各自流水线）
			rwConns = make([]*rawWriteConn, *workers)
			for i := range rwConns {
				rc, err := dialRawWrite(addrs[i%len(addrs)])
				if err != nil {
					log.Fatalf("dial raw write %s: %v", addrs[i%len(addrs)], err)
				}
				defer rc.Close()
				rwConns[i] = rc
			}
			setFn = func(ctx context.Context, wid int, key string, value []byte) error {
				return rwConns[wid%*workers].setBatch([]rawFrame{{key: key, value: value}})
			}
		} else {
			kvClients := make([]proto.KeyValueServiceClient, len(addrs))
			streamClients := make([]proto.KVStreamServiceClient, len(addrs))
			for i, addr := range addrs {
				conn, err := grpc.Dial(addr,
					grpc.WithTransportCredentials(insecure.NewCredentials()),
					grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(32*1024*1024)),
					grpc.WithInitialWindowSize(16*1024*1024),
					grpc.WithInitialConnWindowSize(16*1024*1024),
					grpc.WithWriteBufferSize(4*1024*1024),
					grpc.WithReadBufferSize(4*1024*1024),
				)
				if err != nil {
					log.Fatalf("dial %s: %v", addr, err)
				}
				defer conn.Close()
				kvClients[i] = proto.NewKeyValueServiceClient(conn)
				streamClients[i] = proto.NewKVStreamServiceClient(conn)
			}
			var rr uint64
			pick := func() int { return int(atomic.AddUint64(&rr, 1)-1) % len(addrs) }
			var callOpts []grpc.CallOption
			if *useRaw {
				callOpts = append(callOpts, grpc.ForceCodecV2(proto.RawCodecV2))
			}
			setFn = func(ctx context.Context, _ int, key string, value []byte) error {
				i := pick()
				if *useStream {
					return directSetStream(ctx, streamClients[i], key, value)
				}
				_, err := kvClients[i].Set(ctx, &proto.SetRequest{Key: []byte(key), Value: value}, callOpts...)
				return err
			}
			getFn = func(ctx context.Context, _ int, key string) ([]byte, error) {
				i := pick()
				if *useStream {
					return directGetStream(ctx, streamClients[i], key)
				}
				resp, err := kvClients[i].Get(ctx, &proto.GetRequest{Key: []byte(key)}, callOpts...)
				if err != nil {
					return nil, err
				}
				if !resp.Found {
					return nil, fmt.Errorf("key not found: %s", key)
				}
				return resp.Value, nil
			}
		}
	} else {
		newClient := func(id int) *client.Client {
			cfg := client.DefaultConfig()
			cfg.Node = *node
			cfg.UseRawCodec = *useRaw
			if *sdkDirectAddr != "" {
				// SDK 直连模式：跳过 TiKV
				cfg.DirectAddr = *sdkDirectAddr
				cfg.DirectRawAddr = *sdkDirectRawAddr
			} else {
				cfg.TiKVPD = *tikvPD
				cfg.CACert = *caPath
				cfg.ClientCert = *certPath
				cfg.ClientKey = *keyPath
			}
			sdk, err := client.NewClient(cfg)
			if err != nil {
				log.Fatalf("NewClient(%d): %v", id, err)
			}
			return sdk
		}

		// 共享模式：所有 worker 共用一个 client；独立模式：每 worker 一个 client
		var shared *client.Client
		clients = make([]*client.Client, *workers)
		if *clientPerWorker {
			for i := range clients {
				clients[i] = newClient(i)
			}
		} else {
			shared = newClient(0)
			for i := range clients {
				clients[i] = shared
			}
		}
		defer func() {
			if *clientPerWorker {
				for _, c := range clients {
					c.Close()
				}
			} else {
				shared.Close()
			}
		}()

		setFn = func(ctx context.Context, wid int, key string, value []byte) error {
			if *useStream {
				return clients[wid].SetStream(ctx, key, value)
			}
			return clients[wid].Set(ctx, key, value)
		}
		getFn = func(ctx context.Context, wid int, key string) ([]byte, error) {
			if *useStream {
				return clients[wid].GetStream(ctx, key)
			}
			return clients[wid].Get(ctx, key)
		}

		time.Sleep(3 * time.Second) // 等实例注册表刷新
	}

	var (
		opsOK    int64
		bytesOK  int64
		errCount int64
	)
	var mu sync.Mutex
	latencies := make([]time.Duration, 0, 1<<20)

	deadline := time.Now().Add(*duration)
	start := time.Now()
	var wg sync.WaitGroup

	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func(wid int) {
			defer wg.Done()

			var lat []time.Duration
			if *mode == "read" {
				// 精确遍历读：每个 key 恰好读一次，且读取量 ≤ 写入量。
				// key 空间 = <prefix>:w{0..readWidMod-1}:{0..readMaxSeq-1}（写入时 startSeq=0）。
				// 约束：workers ≤ readWidMod（读 worker 数不超写 worker 数，区间不重叠，key 不重复读）；
				//      readMaxSeq ≤ 写入时 count-per-worker（读取量不超写入量）。
				// 遍历完即结束，不受 duration 限制。
				keyWid := wid % *readWidMod
				// -into：每 worker 一个可复用的出参 buffer，全程零分配
				var intoBuf []byte
				if *useInto {
					intoBuf = make([]byte, *valueSize)
				}
				for seq := 0; seq < *readMaxSeq; seq++ {
					key := fmt.Sprintf("%s:w%d:%d", *prefix, keyWid, seq)
					opStart := time.Now()
					var gotLen int
					var err error
					if *useInto {
						gotLen, err = clients[wid].GetInto(ctx, key, intoBuf)
					} else {
						var got []byte
						got, err = getFn(ctx, wid, key)
						gotLen = len(got)
					}
					if err != nil {
						n := atomic.AddInt64(&errCount, 1)
						if n <= 5 {
							log.Printf("[w%d] Get failed: %v", wid, key)
						}
						continue
					}
					if gotLen != *valueSize {
						atomic.AddInt64(&errCount, 1)
						continue
					}
					lat = append(lat, time.Since(opStart))
				}
			} else {
				// 每 worker 一份随机基础数据；每 op 在前 16 字节嵌入 wid+seq，
				// 避免不同 op 内容相同被去重
				value := make([]byte, *valueSize)
				rand.Read(value)

				// raw-write 批量模式：攒满 batch 帧一次性 sendmmsg，再批量收 ACK
				var pending []rawFrame
				flush := func() {
					if len(pending) == 0 {
						return
					}
					opStart := time.Now()
					if err := rwConns[wid%*workers].setBatch(pending); err != nil {
						atomic.AddInt64(&errCount, 1)
						if atomic.LoadInt64(&errCount) <= 5 {
							log.Printf("[w%d] raw-write batch(%d) failed: %v", wid, len(pending), err)
						}
					} else {
						for range pending {
							lat = append(lat, time.Since(opStart))
						}
					}
					pending = pending[:0]
				}

				// countPerWorker > 0 时按数量写（灌数据用），否则按时间循环
				for seq := *startSeq; time.Now().Before(deadline); seq++ {
					if *countPerWorker > 0 && seq-*startSeq >= *countPerWorker {
						break
					}
					binary.BigEndian.PutUint64(value[:8], uint64(wid))
					binary.BigEndian.PutUint64(value[8:16], uint64(seq))

					key := fmt.Sprintf("%s:w%d:%d", *prefix, wid, seq)
					if *useRawWrite && *batchSize > 1 {
						pending = append(pending, rawFrame{key: key, value: value})
						if len(pending) >= *batchSize {
							flush()
						}
						continue
					}
					opStart := time.Now()
					if err := setFn(ctx, wid, key, value); err != nil {
						atomic.AddInt64(&errCount, 1)
						if atomic.LoadInt64(&errCount) <= 5 {
							log.Printf("[w%d] Set failed: %v", wid, err)
						}
						continue
					}
					lat = append(lat, time.Since(opStart))
				}
				if *useRawWrite && *batchSize > 1 {
					flush() // drain 尾帧
				}
			}

			mu.Lock()
			latencies = append(latencies, lat...)
			mu.Unlock()
			atomic.AddInt64(&opsOK, int64(len(lat)))
			atomic.AddInt64(&bytesOK, int64(len(lat))*int64(*valueSize))
		}(w)
	}
	wg.Wait()
	elapsed := time.Since(start)

	ops := atomic.LoadInt64(&opsOK)
	mb := float64(atomic.LoadInt64(&bytesOK)) / 1024 / 1024
	fmt.Println("\n=== Result ===")
	fmt.Printf("elapsed=%v ops=%d errors=%d\n", elapsed.Round(time.Millisecond), ops, atomic.LoadInt64(&errCount))
	fmt.Printf("data=%.1f GB throughput=%.1f MB/s (%.2f GB/s)\n", mb/1024, mb/elapsed.Seconds(), mb/elapsed.Seconds()/1024)
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		var sum time.Duration
		for _, l := range latencies {
			sum += l
		}
		fmt.Printf("latency avg=%v p50=%v p95=%v p99=%v max=%v\n",
			(sum / time.Duration(len(latencies))).Round(time.Millisecond),
			latencies[len(latencies)*50/100].Round(time.Millisecond),
			latencies[len(latencies)*95/100].Round(time.Millisecond),
			latencies[len(latencies)*99/100].Round(time.Millisecond),
			latencies[len(latencies)-1].Round(time.Millisecond))
	}
	fmt.Printf("note: keys left in place (prefix %s:), reuse for read tests or delete manually\n", *prefix)
}
