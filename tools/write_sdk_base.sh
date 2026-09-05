#!/bin/bash
# SDK 模式 gRPC--raw 基线复现（64 独立客户端连接 + TiKV 索引）
set -e
cd /root
if pgrep -x fio >/dev/null 2>&1; then echo "ABORT: fio running"; exit 1; fi

PIDS=$(pgrep -x kvcache_perf | tr '\n' ' ')
PIDS_CSV=$(echo $PIDS | tr ' ' ',')
TS=$(date +%Y%m%d_%H%M%S)
OUT=/root/wcmp_grpc_sdk_${TS}
mkdir -p $OUT

for pid in $PIDS; do
  [ -f /proc/$pid/io ] && grep '^write_bytes' /proc/$pid/io | awk '{print $2}' > $OUT/io_before_$pid
done

pidstat -d -p $PIDS_CSV 1 > $OUT/pidstat.log 2>&1 &
PIDSTAT_PID=$!
iostat -xm 1 125 > $OUT/iostat.log 2>&1 &
sar -n DEV 1 125 > $OUT/sar.log 2>&1 &

for port in 33001 33021 33031; do
  (sleep 3; curl -s "http://127.0.0.1:$port/debug/pprof/profile?seconds=88" -o $OUT/pprof_server_${port}.pb 2>/dev/null) &
done
(sleep 3; curl -s "http://127.0.0.1:16060/debug/pprof/profile?seconds=88" -o $OUT/pprof_client.pb 2>/dev/null || echo "client pprof skipped") &

sleep 2
START=$(date +%s)
set +e
timeout 115 ./bench --mode write --raw --workers 64 --duration 100s --value-size 4194304 \
  --prefix vwsdk --node 146 > $OUT/bench.log 2>&1
RC=$?
set -e
echo "bench exit=$RC elapsed=$(( $(date +%s) - START ))s"
grep -E 'Result|elapsed|errors|throughput' $OUT/bench.log

sleep 3; kill $PIDSTAT_PID 2>/dev/null; pkill -f 'iostat -xm 1 125' 2>/dev/null; pkill -f 'sar -n DEV 1 125' 2>/dev/null
wait 2>/dev/null

echo "=== /proc/PID/io DELTA ==="
for pid in $PIDS; do
  AFTER=$(grep '^write_bytes' /proc/$pid/io 2>/dev/null | awk '{print $2}') || AFTER=0
  BEFORE=$(cat $OUT/io_before_$pid 2>/dev/null || echo 0)
  echo "pid=$pid: delta=$(( (AFTER - BEFORE)/1024/1024/1024 )) GB"
done
echo "=== iostat wMB/s (skip 12s) ==="
awk '/^nvme5n1 /{d5=$9} /^nvme6n1 /{d6=$9} /^nvme7n1 /{d7=$9} /^nvme5n1 /{n++; if(n>=13){s5+=d5;s6+=d6;s7+=d7;c++; if(d5>p5)p5=d5; if(d6>p6)p6=d6; if(d7>p7)p7=d7; if(d5+d6+d7>pt)pt=d5+d6+d7}} END {printf "AVG TOTAL=%.0f MB/s (%.2f GB/s)  PEAK TOTAL=%.0f MB/s (%.2f GB/s)\n", (s5+s6+s7)/c, (s5+s6+s7)/c/1024, pt, pt/1024}' $OUT/iostat.log
echo "Output: $OUT"