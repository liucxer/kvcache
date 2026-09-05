#!/bin/bash
export PATH=$PATH:/usr/local/go/bin
cd /root/kvcache

PREFIX="single2"
VALUE_SIZE=4194304
WORKERS=80
COUNT=2000
PID=361756

echo "=== WRITE (direct mode, workers=$WORKERS, count=$COUNT, prefix=$PREFIX) ==="
echo "total data = $(( WORKERS * COUNT * VALUE_SIZE / 1024 / 1024 / 1024 )) GB"
START=$(date +%s)
./bench --mode write --raw --direct-addr 127.0.0.1:33000 \
  --workers $WORKERS --duration 400s --value-size $VALUE_SIZE \
  --count-per-worker $COUNT --prefix $PREFIX > /root/write_single2.log 2>&1
RC=$?
echo "write exit=$RC elapsed=$(( $(date +%s) - START ))s"
grep -E 'Result|elapsed|errors|throughput|data=' /root/write_single2.log

echo ""
echo "=== disk usage after write ==="
df -h /mnt/nvme1n1 | tail -1
echo ""
echo "=== verifying key coverage ==="
grep -c 'w[0-9]* seq' /root/write_single2.log 2>/dev/null || echo "check via bench result above"

echo ""
echo "=== READ TEST (80 workers, full monitor) ==="
TS=$(date +%Y%m%d_%H%M%S)
OUT=/root/kvsingle2_${TS}
mkdir -p $OUT

# 权威计数器 before
awk '{print $1, $3}' /sys/block/nvme1n1/stat > $OUT/stat_before
grep '^read_bytes' /proc/$PID/io | awk '{print $2}' > $OUT/io_before

# drop caches
sync; echo 3 > /proc/sys/vm/drop_caches; sleep 2
free -g | head -2

# 实时监控
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

# 读测试
START=$(date +%s)
timeout 70 ./bench --mode read --into \
  --direct-addr 127.0.0.1:33000 \
  --direct-raw-addr 127.0.0.1:29300 \
  --workers $WORKERS --duration 60s --value-size $VALUE_SIZE \
  --prefix $PREFIX --read-max-seq $COUNT --read-wid-mod $WORKERS > $OUT/bench.log 2>&1
RC=$?
echo "read exit=$RC elapsed=$(( $(date +%s) - START ))s"
cat $OUT/bench.log

# 停止监控
sleep 3
kill $IOSTAT_PID $SAR_PID $PIDSTAT_PID $MPSTAT_PID 2>/dev/null
sleep 1
free -g | head -2

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
echo "=== pprof files ==="
ls -la $OUT/pprof_*.pb 2>/dev/null

echo ""
echo "Output: $OUT"
echo "===== SINGLE2 TEST DONE ====="