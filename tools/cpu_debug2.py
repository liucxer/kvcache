#!/usr/bin/env python3
total = 0
bad = 0
with open('/root/kvrd80_full_20260905_033525/mpstat.log') as f:
    for line in f:
        if ' all ' in line and 'CPU' not in line and 'Average' not in line:
            total += 1
            parts = line.split()
            if len(parts) != 13:
                bad += 1
                if bad <= 3:
                    print(f"BAD len={len(parts)}: {line.strip()[:100]}")
print(f"total={total} bad={bad}")

# Also check for commas
with open('/root/kvrd80_full_20260905_033525/mpstat.log') as f:
    for i, line in enumerate(f):
        if ',' in line and ' all ' in line:
            print(f"COMMA at line {i}: {line.strip()[:100]}")
            break