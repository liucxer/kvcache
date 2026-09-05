#!/bin/bash
# Read performance test script - runs on 146
cd /root

PREFIX=${1:-testbw}
DURATION=${2:-15s}
VALUE_SIZE=${3:-4194304}
READ_MAX_SEQ=${4:-100}
READ_WID_MOD=${5:-8}

echo "=========================================="
echo "Read Performance Test"
echo "prefix=$prefix duration=$DURATION value_size=$VALUE_SIZE"
echo "=========================================="

echo ""
echo "=== Path: --into (sendfile zero-copy) ==="
for w in 8 16 32 64 128 256; do
  echo "--- workers=$w ---"
  timeout 30 ./bench --mode read --into --workers $w --duration $DURATION \
    --value-size $VALUE_SIZE --prefix $PREFIX \
    --read-max-seq $READ_MAX_SEQ --read-wid-mod $READ_WID_MOD --node 146 2>&1 \
    | grep -E 'throughput|latency|elapsed'
done

echo ""
echo "=== Path: --direct --raw (gRPC raw codec) ==="
for w in 8 16 32 64; do
  echo "--- workers=$w ---"
  timeout 30 ./bench --mode read --direct 127.0.0.1:33000,127.0.0.1:33020,127.0.0.1:33030 \
    --raw --workers $w --duration $DURATION \
    --value-size $VALUE_SIZE --prefix $PREFIX \
    --read-max-seq $READ_MAX_SEQ --read-wid-mod $READ_WID_MOD --node 146 2>&1 \
    | grep -E 'throughput|latency|elapsed'
done

echo ""
echo "=== Path: --direct (gRPC unary) ==="
for w in 8 16 32 64; do
  echo "--- workers=$w ---"
  timeout 30 ./bench --mode read --direct 127.0.0.1:33000,127.0.0.1:33020,127.0.0.1:33030 \
    --workers $w --duration $DURATION \
    --value-size $VALUE_SIZE --prefix $PREFIX \
    --read-max-seq $READ_MAX_SEQ --read-wid-mod $READ_WID_MOD --node 146 2>&1 \
    | grep -E 'throughput|latency|elapsed'
done

echo ""
echo "=== Path: --direct --stream (gRPC streaming) ==="
for w in 8 16 32 64; do
  echo "--- workers=$w ---"
  timeout 30 ./bench --mode read --direct 127.0.0.1:33000,127.0.0.1:33020,127.0.0.1:33030 \
    --stream --workers $w --duration $DURATION \
    --value-size $VALUE_SIZE --prefix $PREFIX \
    --read-max-seq $READ_MAX_SEQ --read-wid-mod $READ_WID_MOD --node 146 2>&1 \
    | grep -E 'throughput|latency|elapsed'
done

echo ""
echo "=== Done ==="
