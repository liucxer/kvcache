#!/bin/bash
cd ~/nefs/mgmt || exit 1
echo "==== m.port assignments ===="
grep -rn 'm\.port\|s\.port\|\.port =' mgrs/server.go | head
echo "==== checkConfig 145-200 ===="
sed -n '145,200p' mgrs/server.go
echo "==== config keys ===="
grep -nE 'portKey|listenKey|"port"|"listen"|metaPeers' mgrs/*.go | head
