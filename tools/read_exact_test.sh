#!/bin/bash
# 精确遍历读测试：每个 key 恰好读一次，读取量 = 写入量。
# 前提：数据已用 --count-per-worker N 完整写入（errors=0）
cd /root/kvcache
PREFIX=${PREFIX:-exact2}
COUNT=${COUNT:-2400}
WORKERS=${WORKERS:-80}
PID=$(pgrep -x kvcache_perf)
echo "PID=$PID"

TS=$(date +%Y%m%d_%H%M%S)
OUT=/root/kvread_${TS}
mkdir -p $OUT

# 权威计数器 before
awk '{print $1, $3}' /sys/block/nvme1n1/stat > $OUT/stat_before
grep '^read_bytes' /proc/$PID/io | awk '{print $2}' > $OUT/io_before

# 释放 page cache（真实磁盘读）
sync; echo 3 > /proc/sys/vm/drop_caches; sleep 2

# 监控
sar -n DEV 1 500 > $OUT/sar.log 2>&1 &
SAR_PID=$!
pidstat -u -d -p $PID 1 > $OUT/pidstat.log 2>&1 &
PIDSTAT_PID=$!
mpstat -P ALL 1 500 > $OUT/mpstat.log 2>&1 &
MPSTAT_PID=$!

# pprof：服务端 :33000 与客户端 :16060，时长略短于预计读取耗时，结束后 wait
( sleep 3; curl -s "http://127.0.0.1:33001/debug/pprof/profile?seconds=125" -o $OUT/pprof_server.pb 2>/dev/null ) &
PPROF_SRV=$!
( sleep 3; curl -s "http://127.0.0.1:16060/debug/pprof/profile?seconds=125" -o $OUT/pprof_client.pb 2>/dev/null ) &
PPROF_CLI=$!

sleep 2

echo "=== READ TEST ==="
START=$(date +%s)
timeout 600 ./bench --mode read --into \
  --direct-addr 127.0.0.1:33000 --direct-raw-addr 127.0.0.1:29300 \
  --workers $WORKERS --duration 60s --value-size 4194304 \
  --prefix $PREFIX --read-max-seq $COUNT --read-wid-mod $WORKERS > $OUT/bench.log 2>&1
RC=$?
echo "read exit=$RC elapsed=$(( $(date +%s) - START ))s"
cat $OUT/bench.log

# 停止监控并 wait pprof 完成
sleep 3
kill $SAR_PID $PIDSTAT_PID $MPSTAT_PID 2>/dev/null
wait $PPROF_SRV $PPROF_CLI 2>/dev/null
sleep 1

# 磁盘真实读 delta
echo ""
echo "=== /sys/block stat DELTA ==="
SB=$(awk '{print $2}' $OUT/stat_before)
SA=$(awk '{print $3}' /sys/block/nvme1n1/stat)
GB=$(( (SA - SB) * 512 / 1024 / 1024 / 1024 ))
echo "nvme1n1: ~${GB} GB (read sectors delta)"

echo ""
echo "=== /proc/PID/io DELTA ==="
AFTER=$(grep '^read_bytes' /proc/$PID/io | awk '{print $2}')
BEFORE=$(cat $OUT/io_before)
echo "pid=$PID: $(( (AFTER - BEFORE)/1024/1024/1024 )) GB"

echo ""
echo "=== sar lo avg ==="
grep ' lo ' $OUT/sar.log | awk 'NR>8{s4+=$4;s5+=$5;c++} END {printf "samples=%d lo_rx=%.2f GB/s lo_tx=%.2f GB/s\n", c, s4/c/1024/1024, s5/c/1024/1024}'

echo ""
echo "=== mpstat avg ==="
grep -E 'Average' $OUT/mpstat.log | tail -1

echo ""
echo "Output: $OUT"
echo "===== READ TEST DONE ====="
