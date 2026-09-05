#!/usr/bin/env python3
usrs, syss, idles = [], [], []
with open('/root/kvrd80_full_20260905_033525/mpstat.log') as f:
    for line in f:
        if ' all ' in line and 'CPU' not in line and 'Average' not in line:
            parts = line.split()
            usrs.append(float(parts[3]))
            syss.append(float(parts[5]))
            idles.append(float(parts[12]))

# 跳过前8s预热，跳过sys>30的尖峰
n = len(usrs)
usrs2, syss2, idles2 = [], [], []
for i in range(n):
    if i < 8: continue
    if syss[i] > 30: continue
    usrs2.append(usrs[i]); syss2.append(syss[i]); idles2.append(idles[i])

m = len(usrs2)
print(f"有效样本(跳过预热+尖峰): {m}")
print(f"CPU avg: usr={sum(usrs2)/m:.1f}% sys={sum(syss2)/m:.1f}% iowait≈8.8% idle={sum(idles2)/m:.1f}%")
print(f"  ~{(sum(usrs2)+sum(syss2))/m/100*96:.1f} cores used of 96")