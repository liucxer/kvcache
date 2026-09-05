#!/bin/bash
# 确认实例 PID 和 name 的对应关系
for pid in $(pgrep -x kvcache_perf); do
  name=$(cat /proc/$pid/cmdline 2>/dev/null | tr '\0' ' ' | grep -oP 'name \K[^ ]+')
  node=$(cat /proc/$pid/cmdline 2>/dev/null | tr '\0' ' ' | grep -oP 'node \K[^ ]+')
  grpc=$(cat /proc/$pid/cmdline 2>/dev/null | tr '\0' ' ' | grep -oP 'grpc-port \K[^ ]+')
  echo "pid=$pid name=$name node=$node grpc=$grpc"
done