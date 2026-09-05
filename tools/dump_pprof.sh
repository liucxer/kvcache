#!/bin/bash
# 下载 pprof 文件并导出 base64
OUT=/root/kvread_20260904_232212
for f in pprof_client.pb pprof_server_33001.pb pprof_server_33021.pb pprof_server_33041.pb; do
  echo "===BEGIN:$f==="
  base64 -w0 $OUT/$f
  echo ""
  echo "===END:$f==="
done