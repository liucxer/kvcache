#!/bin/bash
# Monitored read performance test for 146
# Measures real disk-read bandwidth with page cache cleared.
# Runs iostat + network monitoring + pprof in parallel with bench.

set -e
cd /root

PREFIX=${1:-bigdata}
DURATION=${2:-30s}
VALUE_SIZE=${3:-4194304}
READ_MAX_SEQ=${4:-1200}
READ_WID_MOD=${5:-64}
WORKERS=${6:-64}

OUTDIR=/root/perf_test_$(date +%s)
mkdir -p $OUTDIR
echo "Output dir: $OUTDIR"

# ---- 0. Check write/read raw NVMe baseline with fio (quick) ----
echo "=== Quick fio baseline (10s each) ==="
for dev in nvme5n1 nvme6n1 nvme7n1; do
  echo "--- $dev randread ---"
  fio --name=randread_$dev --filename=/dev/$dev --rw=randread \
    --bs=4m --iodepth=64 --numjobs=4 --runtime=10 --time_based \
    --direct=1 --ioengine=libaio --group_reporting \
    --output-format=terse 2>/dev/null | grep -E "READ:|IOPS" || echo "fio not available or failed"
done

# ---- 1. Write large dataset if not exists ----
echo ""
echo "=== Checking/writing large dataset: prefix=$PREFIX ==="
# Check if data exists by probing a few keys
EXISTS=$(curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:33001/api/v1/kv/${PREFIX}:w0:0 2>/dev/null || echo "000")
if [ "$EXISTS" != "200" ]; then
  echo "Data not found, writing 64 workers x 1200 keys x 4MB = ~300GB..."
  # Monitor write phase too
  iostat -xm 1 5 > $OUTDIR/write_iostat.log 2>&1 &
  IOSTAT_PID=$!
  sar -n DEV 1 5 > $OUTDIR/write_net.log 2>&1 &
  SAR_PID=$!

  timeout 600 ./bench --mode write --workers 64 --count-per-worker 1200 \
    --value-size $VALUE_SIZE --prefix $PREFIX --node 146 2>&1 | grep -E 'Result|elapsed|data=|note:'

  wait $IOSTAT_PID 2>/dev/null || true
  wait $SAR_PID 2>/dev/null || true
  echo "Write iostat summary:"
  grep -E "nvme5|nvme6|nvme7" $OUTDIR/write_iostat.log | tail -10
else
  echo "Data already exists (HTTP $EXISTS), skipping write"
fi

# ---- 2. Read test with full monitoring ----
echo ""
echo "=========================================="
echo "READ TEST: workers=$WORKERS duration=$DURATION prefix=$PREFIX"
echo "=========================================="

# Clear page cache to force disk reads
echo "=== Dropping page cache ==="
sync
echo 3 > /proc/sys/vm/drop_caches
sleep 2
echo "Page cache dropped. Available memory:"
free -g | head -2

# Start background monitors
echo "Starting background monitors..."
iostat -xm 1 > $OUTDIR/read_iostat.log 2>&1 &
IOSTAT_PID=$!
sar -n DEV 1 > $OUTDIR/read_net.log 2>&1 &
SAR_PID=$!

# Try pprof (bench listens on :16060 for client pprof)
# Use curl to collect profile if available
PPROF_PID=""
sleep 1  # let bench start pprof server
(
  sleep 2  # wait for bench to start
  curl -s "http://127.0.0.1:16060/debug/pprof/profile?seconds=25" -o $OUTDIR/pprof_cpu.pb 2>/dev/null || echo "pprof failed" > $OUTDIR/pprof_status.txt
) &
PPROF_PID=$!

# Run bench
echo "Starting bench..."
START_TIME=$(date +%s)
timeout 60 ./bench --mode read --into --workers $WORKERS --duration $DURATION \
  --value-size $VALUE_SIZE --prefix $PREFIX \
  --read-max-seq $READ_MAX_SEQ --read-wid-mod $READ_WID_MOD --node 146 \
  > $OUTDIR/bench_output.log 2>&1
END_TIME=$(date +%s)
echo "Bench finished in $((END_TIME - START_TIME))s"

# Stop monitors
sleep 2
kill $IOSTAT_PID 2>/dev/null || true
kill $SAR_PID 2>/dev/null || true
wait $IOSTAT_PID 2>/dev/null || true
wait $SAR_PID 2>/dev/null || true
[ -n "$PPROF_PID" ] && wait $PPROF_PID 2>/dev/null || true

# ---- 3. Collect and summarize ----
echo ""
echo "=== BENCH RESULT ==="
grep -E 'Result|elapsed|data=|latency|note:' $OUTDIR/bench_output.log

echo ""
echo "=== DISK I/O SUMMARY (iostat) ==="
echo "Peak read bandwidth per NVMe device:"
echo "device   rMB/s_avg   rMB/s_peak   %util_avg"
for dev in nvme5n1 nvme6n1 nvme7n1; do
  # Extract rkB/s and %util, convert to MB/s
  STATS=$(awk -v dev="$dev" '$1==dev {rkb+=$6; util+=$NF; cnt++; if($6>max) max=$6} END {printf "%.1f %.1f %.1f", rkb/cnt/1024, max/1024, util/cnt}' $OUTDIR/read_iostat.log 2>/dev/null)
  echo "$dev  $STATS"
done

echo ""
echo "=== NETWORK I/O SUMMARY (sar) ==="
echo "Peak RX/TX bandwidth:"
for iface in $(ls /sys/class/net/ 2>/dev/null | grep -v lo); do
  STATS=$(awk -v iface="$iface" '$2==iface {rx+=$3; tx+=$4; cnt++; if($3>rxmax) rxmax=$3; if($4>txmax) txmax=$4} END {if(cnt>0) printf "%s: rx_avg=%.1f MB/s tx_avg=%.1f MB/s rx_peak=%.1f tx_peak=%.1f\n", iface, rx/8/cnt, tx/8/cnt, rxmax/8, txmax/8}' $OUTDIR/read_net.log 2>/dev/null)
  [ -n "$STATS" ] && echo "$STATS"
done

echo ""
echo "=== CPU USAGE DURING TEST ==="
# Take snapshot
top -bn1 | head -5

echo ""
echo "=== OUTPUT FILES ==="
ls -la $OUTDIR/
echo "Output dir: $OUTDIR"

echo ""
echo "=== DONE ==="
