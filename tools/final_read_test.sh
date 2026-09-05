#!/bin/bash
cd /root/kvcache
PREFIX="clean1"
COUNT=1000
WORKERS=80
PID=$(pgrep -x kvcache_perf)
echo "PID=$PID"

TS=$(date +%Y%m%d_%H%M%S)
OUT=/root/kvfinal_${TS}
mkdir -p $OUT

# 权威计数器 before
awk '{print $1, $3}' /sys/block/nvme1n1/stat > $OUT/stat_before
grep '^read_bytes' /proc/$PID/io | awk '{print $2}' > $OUT/io_before

# drop caches
sync; echo 3 > /proc/sys/vm/drop_caches; sleep 2
free -g | head -2

# 监控
iostat -xm 1 75 > $OUT/iostat.log 2>&1 &
IOSTAT_PID=$!
sar -n DEV 1 75 > $OUT/sar.log 2>&1 &
SAR_PID=$!
pidstat -u -d -p $PID 1 > $OUT/pidstat.log 2>&1 &
PIDSTAT_PID=$!
mpstat -P ALL 1 75 > $OUT/mpstat.log 2>&1 &
MPSTAT_PID=$!

# pprof
(sleep 3; curl -s "http://127.0.0.1:33001/debug/pprof/profile?seconds=57" -o $OUT/pprof_server.pb 2>/dev/null) &
(sleep 3; curl -s "http://127.0.0.1:16060/debug/pprof/profile?seconds=57" -o $OUT/pprof_client.pb 2>/dev/null || echo "no client pprof") &

sleep 2

echo "=== READ TEST ==="
START=$(date +%s)
timeout 70 ./bench --mode read --into \
  --direct-addr 127.0.0.1:33000 \
  --direct-raw-addr 127.0.0.1:29300 \
  --workers $WORKERS --duration 60s --value-size 4194304 \
  --prefix $PREFIX --read-max-seq $COUNT --read-wid-mod $WORKERS > $OUT/bench.log 2>&1
RC=$?
echo "read exit=$RC elapsed=$(( $(date +%s) - START ))s"
cat $OUT/bench.log

# 停止监控
sleep 3
kill $IOSTAT_PID $SAR_PID $PIDSTAT_PID $MPSTAT_PID 2>/dev/null
sleep 1

# 权威计数器 delta
echo ""
echo "=== /sys/block stat DELTA ==="
SB=$(awk '{print $2}' $OUT/stat_before)
SA=$(awk '{print $3}' /sys/block/nvme1n1/stat)
GB=$(( (SA - SB) * 512 / 1024 / 1024 / 1024 ))
echo "nvme1n1: ~${GB} GB -> ~$(( GB / 60 )) GB/s"

echo ""
echo "=== /proc/PID/io DELTA ==="
AFTER=$(grep '^read_bytes' /proc/$PID/io | awk '{print $2}')
BEFORE=$(cat $OUT/io_before)
echo "pid=$PID: $(( (AFTER - BEFORE)/1024/1024/1024 )) GB"

echo ""
echo "=== sar lo avg (skip 8s) ==="
grep ' lo ' $OUT/sar.log | awk 'NR>8{s4+=$4;s5+=$5;c++} END {printf "samples=%d\nlo_rx_avg=%.0f MB/s (%.2f GB/s)\nlo_tx_avg=%.0f MB/s (%.2f GB/s)\n", c, s4/c/1024, s4/c/1024/1024, s5/c/1024, s5/c/1024/1024}'

echo ""
echo "=== mpstat avg ==="
grep -E 'Average|all' $OUT/mpstat.log | tail -1

echo ""
echo "Output: $OUT"
echo "===== READ TEST DONE ====="