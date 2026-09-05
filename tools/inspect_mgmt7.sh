#!/bin/bash
cd ~/nefs/mgmt || exit 1
echo "==== api_service_endpoint.go ===="
sed -n '1,120p' mgrs/api_service_endpoint.go
echo "==== ListenPort const ===="
grep -rn 'ListenPort' proto/ mgrs/ | head
