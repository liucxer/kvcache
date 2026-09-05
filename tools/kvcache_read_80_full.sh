#!/bin/bash
# 128.12 kvcache 读性能全量监控：80 workers
set -e
cd /root

PIDS=$(pgrep -x kvcache_perf | tr '\n' ' ')
PIDS_CSV=$(echo $PIDS | tr ' ' ',')
if [ -z "$PIDS" ]; then echo "ABORT: no kvcache"; exit 1; fi
echo "kvcache PIDs: $PIDS"

TS=$(date +%Y%m%d_%H%M%S)
OUT=/root/kvrd80_full_${TS}
mkdir -p $OUT
echo "OUT=$OUT"

# === 权威计数器 before ===
for d in nvme1n1 nvme2n1 nvme3n1; do
  awk '{print $1, $3}' /sys/block/$d/stat > $OUT/stat_before_$d
done
for pid in $PIDS; do
  grep '^read_bytes' /proc/$pid/io | awk '{print $2}' > $OUT/io_before_$pid
done

# === drop caches ===
sync; echo 3 > /proc/sys/vm/drop_caches; sleep 2
free -g | head -2

# === 实时监控（1s 采样，75次） ===
echo "Starting real-time monitors..."
iostat -xm 1 75 > $OUT/iostat.log 2>&1 &
IOSTAT_PID=$!
sar -n DEV 1 75 > $OUT/sar_net.log 2>&1 &
SAR_PID=$!
pidstat -u -d -p $PIDS_CSV 1 > $OUT/pidstat.log 2>&1 &
PIDSTAT_PID=$!
mpstat -P ALL 1 75 > $OUT/mpstat.log 2>&1 &
MPSTAT_PID=$!
top -b -d 1 -n 75 > $OUT/top.log 2>&1 &
TOP_PID=$!

# 网络字节级高精度采样
(for i in $(seq 1 75); do
  echo "=== $i ==="
  awk '/^  lo:/{print "lo rx="$2" tx="$10}' /proc/net/dev
  awk '/^eth0:/{print "eth0 rx="$2" tx="$10}' /proc/net/dev
  sleep 1
done) > $OUT/net_bytes.log 2>&1 &
NET_PID=$!

# === pprof（覆盖整个读测试） ===
for port in 33001 33021 33041; do
  (sleep 3; curl -s "http://127.0.0.1:$port/debug/pprof/profile?seconds=57" -o $OUT/pprof_server_${port}.pb 2>/dev/null) &
  eval "SERVER_PID_${port}=$!"
done
(sleep 3; curl -s "http://127.0.0.1:16060/debug/pprof/profile?seconds=57" -o $OUT/pprof_client.pb 2>/dev/null || echo "client pprof failed") &
CLIENT_PPROF_PID=$!

sleep 2

# === 读测试 ===
echo "=== READ (80 workers) ==="
START=$(date +%s)
set +e
timeout 70 ./bench --mode read --into --workers 80 --duration 60s --value-size 4194304 \
  --prefix rdperf --read-max-seq 4600 --read-wid-mod 64 --node 128.12 \
  --tikv-pd 127.0.0.1:2379 --ca= --cert= --key= > $OUT/bench.log 2>&1
RC=$?
set -e
echo "bench exit=$RC elapsed=$(( $(date +%s) - START ))s"
grep -E 'Result|elapsed|errors|throughput|latency' $OUT/bench.log

# === 停止监控 ===
sleep 3
kill $IOSTAT_PID $SAR_PID $PIDSTAT_PID $MPSTAT_PID $TOP_PID $NET_PID 2>/dev/null
wait 2>/dev/null || true
free -g | head -2

# === 权威计数器 after & delta ===
echo ""
echo "=== /sys/block stat DELTA ==="
TOTAL_GB=0
for d in nvme1n1 nvme2n1 nvme3n1; do
  RB_B=$(awk '{print $1}' $OUT/stat_before_$d)
  SB_B=$(awk '{print $2}' $OUT/stat_before_$d)
  RA=$(awk '{print $1}' /sys/block/$d/stat)
  SA=$(awk '{print $3}' /sys/block/$d/stat)
  GB=$(( (SA - SB_B) * 512 / 1024 / 1024 / 1024 ))
  TOTAL_GB=$((TOTAL_GB + GB))
  echo "$d: read_sectors=$((SA - SB_B)) (~${GB} GB)"
done
echo "TOTAL disk read: ${TOTAL_GB} GB -> ~$(( TOTAL_GB / 60 )) GB/s"

echo ""
echo "=== /proc/PID/io DELTA ==="
TOTAL_PROC=0
for pid in $PIDS; do
  AFTER=$(grep '^read_bytes' /proc/$pid/io 2>/dev/null | awk '{print $2}') || AFTER=0
  BEFORE=$(cat $OUT/io_before_$pid 2>/dev/null || echo 0)
  GB=$(( (AFTER - BEFORE)/1024/1024/1024 )); TOTAL_PROC=$((TOTAL_PROC + GB))
  echo "pid=$pid: $GB GB"
done
echo "TOTAL proc read: ${TOTAL_PROC} GB"

echo ""
echo "=== iostat 各盘读带宽 (avg + peak, 跳过前8s) ==="
awk '
/^nvme1n1 /{r1=$2; n++; if(n>=9){s1+=r1; if(r1>p1)p1=r1; c++}}
/^nvme2n1 /{r2=$2;       m++; if(m>=9){s2+=r2; if(r2>p2)p2=r2}}
/^nvme3n1 /{r3=$2;       k++; if(k>=9){s3+=r3; if(r3>p3)p3=r3}}
END {
  printf "samples=%d\n", c
  printf "AVG  nvme1=%.0f nvme2=%.0f nvme3=%.0f TOTAL=%.0f MB/s (%.2f GB/s)\n", s1/c, s2/c, s3/c, (s1+s2+s3)/c, (s1+s2+s3)/c/1024
  printf "PEAK nvme1=%.0f nvme2=%.0f nvme3=%.0f TOTAL=%.0f MB/s (%.2f GB/s)\n", p1, p2, p3, p1+p2+p3, (p1+p2+p3)/1024
}' $OUT/iostat.log

echo ""
echo "=== sar lo 网络 (avg + peak, 跳过前8s) ==="
grep ' lo ' $OUT/sar_net.log | awk 'NR>8{rx=$5; tx=$6; srx+=rx; stx+=tx; if(rx>prx)prx=rx; if(tx>ptx)ptx=tx; c++} END {printf "samples=%d\nlo_avg_rx=%.0f MB/s (%.2f GB/s)  peak_rx=%.0f MB/s (%.2f GB/s)\nlo_avg_tx=%.0f MB/s (%.2f GB/s)  peak_tx=%.0f MB/s (%.2f GB/s)\n", c, srx/c/1024, srx/c/1024/1024, prx/1024, prx/1024/1024, stx/c/1024, stx/c/1024/1024, ptx/1024, ptx/1024/1024}'

echo ""
echo "=== pidstat CPU (avg, 跳过前8s) ==="
awk '/kvcache_perf/{n++; if(n>=9){u+=$8; c++}} END {if(c>0)printf "kvcache_perf avg %%CPU=%.1f (%%user=%.1f)\n", u/c, u/c}' $OUT/pidstat.log

echo ""
echo "=== mpstat CPU 总体 (avg, 跳过前8s) ==="
awk '/Average:/{gsub(/,/, "."); print}' $OUT/mpstat.log | head -5

echo ""
echo "=== pprof files ==="
ls -la $OUT/pprof_*.pb 2>/dev/null

echo ""
echo "Output: $OUT"
echo "===== FULL MONITOR DONE ====="