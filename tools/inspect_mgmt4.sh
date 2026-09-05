#!/bin/bash
cd ~/nefs/mgmt || exit 1
echo "==== server.go 60-130 ===="
sed -n '60,130p' mgrs/server.go
echo "==== server.go 140-200 ===="
sed -n '140,200p' mgrs/server.go
echo "==== raft port from peers ===="
grep -rn 'peerPort\|Port()\|m.port\|s.port\|\.port' mgrs/server.go | head -20
