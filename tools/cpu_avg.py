#!/usr/bin/env python3
import re

usrs, syss, iows, idles = [], [], [], []
with open('/root/kvrd80_full_20260905_033525/mpstat.log') as f:
    for line in f:
        if ' all ' in line and 'CPU' not in line and 'Average' not in line:
            parts = line.split()
            if len(parts) >= 13:
                try:
                    usrs.append(float(parts[3]))
                    syss.append(float(parts[5]))
                    iows.append(float(parts[6]))
                    idles.append(float(parts[12]))
                except ValueError:
                    pass

n = len(usrs)
if n:
    print(f"samples={n}")
    print(f"CPU avg: usr={sum(usrs)/n:.1f}% sys={sum(syss)/n:.1f}% iowait={sum(iows)/n:.1f}% idle={sum(idles)/n:.1f}%")
    print(f"  ~{(sum(usrs)+sum(syss))/n/100*96:.1f} cores used of 96")