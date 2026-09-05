#!/bin/bash
# 单实例写性能测试：raw-write (sendmsg) 批量写，采集磁盘/网络/CPU/pprof
cd /root/kvcache
PREFIX=${PREFIX:-wrtest1}
WORKERS=${WORKERS:-80}
DURATION=${DURATION:-60s}
PID=$(pgrep -x kvcache_perf)
echo "PID=$PID"

TS=$(date +%Y%m%d_%H%M%S)
OUT=/root/kvwrite_${TS}
mkdir -p $OUT

# 权威计数器 before（field7 = sectors written）
awk '{print $7}' /sys/block/nvme1n1/stat > $OUT/stat_before
grep '^write_bytes' /proc/$PID/io | awk '{print $2}' > $OUT/io_before

# 监控
sar -n DEV 1 90 > $OUT/sar.log 2>&1 &
SAR_PID=$!
pidstat -u -d -p $PID 1 > $OUT/pidstat.log 2>&1 &
PIDSTAT_PID=$!
mpstat -P ALL 1 90 > $OUT/mpstat.log 2>&1 &
MPSTAT_PID=$!

# pprof：服务端 :33001 与客户端 :16060，时长略短于写耗时
( sleep 3; curl -s "http://127.0.0.1:33001/debug/pprof/profile?seconds=55" -o $OUT/pprof_server.pb 2>/dev/null ) &
PPROF_SRV=$!
( sleep 3; curl -s "http://127.0.0.1:16060/debug/pprof/profile?seconds=55" -o $OUT/pprof_client.pb 2>/dev/null ) &
PPROF_CLI=$!

sleep 2

echo "=== WRITE TEST ==="
START=$(date +%s)
timeout 120 ./bench --mode write --direct 127.0.0.1:29301 --raw-write --batch 8 \
  --workers $WORKERS --duration $DURATION --value-size 4194304 --prefix $PREFIX > $OUT/bench.log 2>&1
RC=$?
echo "write exit=$RC elapsed=$(( $(date +%s) - START ))s"
cat $OUT/bench.log

# 停止监控并 wait pprof
sleep 3
kill $SAR_PID $PIDSTAT_PID $MPSTAT_PID 2>/dev/null
wait $PPROF_SRV $PPROF_CLI 2>/dev/null
sleep 1

# 磁盘真实写 delta（field7 = sectors written，*512 bytes）
echo ""
echo "=== /sys/block stat DELTA (write) ==="
WB=$(cat $OUT/stat_before)
WA=$(awk '{print $7}' /sys/block/nvme1n1/stat)
GB=$(( (WA - WB) * 512 / 1024 / 1024 / 1024 ))
echo "nvme1n1 write: ~${GB} GB"

echo ""
echo "=== /proc/PID/io DELTA (write_bytes) ==="
AFTER=$(grep '^write_bytes' /proc/$PID/io | awk '{print $2}')
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
echo "===== WRITE TEST DONE ====="
