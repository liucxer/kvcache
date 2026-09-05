#!/bin/bash
for p in $(pgrep -x kvcache_perf); do
  echo "PID=$p"
  tr '\0' '\n' < /proc/$p/cmdline | grep -iE 'pprof|addr|port|http'
  echo "---"
done