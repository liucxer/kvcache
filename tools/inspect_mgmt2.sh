#!/bin/bash
cd ~/nefs/mgmt || exit 1
echo "==== http_server.go head ===="
sed -n '1,80p' http_server.go
echo "==== port/listen in config ===="
grep -nE '"port"|"listen"|GetString\("port"\)|GetString\("listen"\)|addr|:12379|:8081' mgrs/config.go | head -30
