#!/bin/bash
docker ps -a | head -25
echo "==== PD/TIKV CONTAINER ===="
for c in $(docker ps -aq); do
  name=$(docker inspect -f '{{.Name}}' "$c" 2>/dev/null)
  img=$(docker inspect -f '{{.Config.Image}}' "$c" 2>/dev/null)
  cmd=$(docker inspect -f '{{.Config.Cmd}}' "$c" 2>/dev/null)
  case "$name" in
    *pd*|*tikv*|*ukv*|*meta*) echo "$name | $img | $cmd" ;;
  esac
done
echo "==== NETWORK NS of pd-server ===="
for pid in $(pgrep -f pd-server); do
  echo "pd pid=$pid ns=$(readlink /proc/$pid/ns/pid)"
done
for pid in $(pgrep -f tikv-server); do
  echo "tikv pid=$pid ns=$(readlink /proc/$pid/ns/pid)"
done
