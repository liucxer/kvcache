#!/bin/bash
# Extract results from the perf test
OUTDIR=$(ls -d /root/perf_* 2>/dev/null | tail -1)
echo "=== Output dir: $OUTDIR ==="

echo ""
echo "=== Bench Read Result ==="
grep -E 'elapsed|data=|latency' $OUTDIR/bench_read.log 2>/dev/null

echo ""
echo "=== pidstat sample (non-zero kB_rd/s) ==="
awk '$4>0 && $4!="kB_rd/s" {print $0}' $OUTDIR/pidstat.log 2>/dev/null | head -10

echo ""
echo "=== pidstat average per PID ==="
awk '$3~/[0-9]+/ && $4>0 {sum[$3]+=$4; cnt[$3]++} END {for(p in sum) print "pid="p" avg="sum[p]/cnt[p]" kB/s"}' $OUTDIR/pidstat.log 2>/dev/null

echo ""
echo "=== iostat per disk (non-idle) ==="
grep -E 'nvme' $OUTDIR/iostat.log 2>/dev/null | grep -v 'Device' | head -20

echo ""
echo "=== Write result ==="
grep -E 'elapsed|data=|errors' $OUTDIR/bench_write.log 2>/dev/null

echo ""
echo "=== pprof file ==="
ls -la $OUTDIR/pprof_cpu.pb 2>/dev/null