#!/bin/bash
echo "==== ukv_pd inspect ===="
docker inspect ukv_pd --format 'NetworkMode={{.HostConfig.NetworkMode}} Ports={{.HostConfig.PortBindings}} IP={{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}} Mounts={{range .Mounts}}{{.Source}}->{{.Destination}};{{end}}'
echo "==== ukv_tikv inspect ===="
docker inspect ukv_tikv --format 'NetworkMode={{.HostConfig.NetworkMode}} Ports={{.HostConfig.PortBindings}} IP={{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}} Mounts={{range .Mounts}}{{.Source}}->{{.Destination}};{{end}}'
echo "==== tikv_make inspect ===="
docker inspect tikv_make --format 'NetworkMode={{.HostConfig.NetworkMode}} Mounts={{range .Mounts}}{{.Source}}->{{.Destination}};{{end}}'
echo "==== existing tikv listen ports ===="
ss -tlnp 2>/dev/null | grep -E '2379|2380|20160|2389|2390|20170' | head
