#!/bin/bash
# 128.12 上创建 kvcache pool + target + vol
export PATH=/usr/bin:/usr/sbin:/usr/local/bin:$PATH
M=127.0.0.1:8081

POOL=$(uuidgen)
echo "POOL=$POOL"
nefs.tools pool add --master $M --pool "$POOL" \
  --desc 'kvcache storage pool' --storage kvcache \
  --bucket 100.71.128.12:33000 \
  --extra '{"rawReadAddr":"100.71.128.12:29300","rawWriteAddr":"100.71.128.12:29301","httpAddr":"100.71.128.12:33001"}' \
  --output json 2>&1
echo "==== POOL LIST ===="
nefs.tools pool list --master $M --output json 2>&1

echo "==== TARGET SET ===="
nefs.tools target set --master $M --name nefs-target --pid 1 --pool "$POOL" --desc "kvcache target" --output json 2>&1
echo "==== TARGET LIST ===="
nefs.tools target list --master $M --output json 2>&1

VOL=$(uuidgen)
echo "VOL=$VOL"
echo "==== VOL CREATE ===="
nefs.tools vol create --master $M --name "$VOL" \
  --capacity 1000 --inodes 100000 --block-size 4096 \
  --target nefs-target --compress none --dir-stats --output json 2>&1
echo "==== VOL LIST ===="
nefs.tools vol list --master $M --output json 2>&1
echo "$VOL" > /root/nefs_vol.txt
echo "$POOL" > /root/nefs_pool.txt
