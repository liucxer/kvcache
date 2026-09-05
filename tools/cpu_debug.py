#!/usr/bin/env python3
with open('/root/kvrd80_full_20260905_033525/mpstat.log') as f:
    count = 0
    for line in f:
        if ' all ' in line and 'CPU' not in line and 'Average' not in line:
            parts = line.split()
            print(f"len={len(parts)} parts={parts[:14]}")
            count += 1
            if count >= 5:
                break