#!/bin/bash
export PATH=$PATH:/usr/local/go/bin
echo "=== kvcache_perf processes ==="
for pid in $(pgrep -x kvcache_perf); do
  cmdline=$(cat /proc/$pid/cmdline 2>/dev/null | tr '\0' ' ')
  echo "pid=$pid cmd=$cmdline"
done
echo ""
echo "=== ports ==="
ss -tlnp | grep -E '3300[0-9]|3302[0-9]|3304[0-9]|2930[0-9]|2932[0-9]|2934[0-9]' || netstat -tlnp | grep -E '3300|2930|2932|2934'