#!/bin/bash
cd ~/nefs/mgmt || exit 1
echo "==== cmd_master.go listen refs ===="
grep -n 'listen' cmd_master.go | head -20
echo "==== HTTP listen ===="
grep -nE 'ListenAndServe|net.Listen|:80|:8081|:12379' http_server.go cmd_master.go cmd_main.go | head -30
echo "==== master flags/config ===="
grep -rn 'metaPeers\|metaTLS' --include='*.go' . | grep -v _test | head -20
echo "==== liveness route ===="
grep -rn 'liveness' --include='*.go' . | grep -v _test | head
