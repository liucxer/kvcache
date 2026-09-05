#!/bin/bash
cd ~/nefs/mgmt || exit 1
echo "==== checkConfig ===="
sed -n '200,270p' mgrs/server.go
