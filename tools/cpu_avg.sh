#!/bin/bash
F=/root/kvrd80_full_20260905_033525/mpstat.log
grep ' all ' $F | awk '{usr+=$4; sys+=$6; iow+=$7; idle+=$13; n++} END {printf "samples=%d\nCPU avg: usr=%.1f%% sys=%.1f%% iowait=%.1f%% idle=%.1f%%  (~%.1f cores used of 96)\n", n, usr/n, sys/n, iow/n, idle/n, (usr+sys)/100*96}'