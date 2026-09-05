#!/usr/bin/env python3
usrs, syss, idles = [], [], []
with open('/root/kvrd80_full_20260905_033525/mpstat.log') as f:
    for line in f:
        if ' all ' in line and 'CPU' not in line and 'Average' not in line:
            parts = line.split()
            usrs.append(float(parts[3]))
            syss.append(float(parts[5]))
            idles.append(float(parts[12]))

print(f"usr: min={min(usrs):.1f} max={max(usrs):.1f} avg={sum(usrs)/len(usrs):.1f}")
print(f"sys: min={min(syss):.1f} max={max(syss):.1f} avg={sum(syss)/len(syss):.1f}")
print(f"idle: min={min(idles):.1f} max={max(idles):.1f} avg={sum(idles)/len(idles):.1f}")
# Check if sys max is the problem
if max(syss) > 10:
    idx = syss.index(max(syss))
    print(f"max sys at line {idx}: usr={usrs[idx]} sys={syss[idx]} idle={idles[idx]}")