#!/bin/bash
cd ~/nefs/mgmt || exit 1
echo "==== Server.Port / NewServer ===="
grep -rn 'func (.*) Port()\|func NewServer\|Port = \|port :=\|GetString("port")\|GetString("listen")' mgrs/*.go | head -20
echo "==== config default port ===="
grep -rnE 'port|listen|12379|8081' mgrs/config.go | head -30
echo "==== where listen config used ===="
grep -rn 'listen' --include='*.go' mgrs/ cmd_master.go | head -20
