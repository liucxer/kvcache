#!/bin/bash
# Download pprof files as base64 with name markers
OUTDIR=/root/perf_wr_1788514130
for f in pprof_client.pb pprof_server_33001.pb pprof_server_33021.pb pprof_server_33031.pb; do
  if [ -f "$OUTDIR/$f" ]; then
    echo "###MARKER_${f}###"
    base64 -w0 "$OUTDIR/$f"
    echo ""
  fi
done