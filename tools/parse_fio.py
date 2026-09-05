import json
d = json.load(open('/root/fio_result/write_nvme1.json'))
j = d['jobs'][0]
w = j['write']
print("=== fio write nvme1n1 ===")
print("job elapsed:", j['elapsed'], "s")
print("error:", j['error'])
print("io_bytes:", w['io_bytes'], "(%.1f GB)" % (w['io_bytes']/1024/1024/1024))
print("bw:", w['bw'], "(%.0f MB/s, %.2f GB/s)" % (w['bw']/1024/1024, w['bw']/1024/1024/1024))
print("iops:", w['iops'])
print("runtime:", w['runtime'], "ms")
print("total_ios:", w['total_ios'])
print("short_ios:", w['short_ios'])
print("drop_ios:", w['drop_ios'])
print("slat mean: %.1f us" % (w['slat_ns']['mean']/1000))
print("clat mean: %.1f us" % (w['clat_ns']['mean']/1000))
print("lat mean: %.1f us" % (w['lat_ns']['mean']/1000))
p = w['clat_ns']['percentile']
print("clat p50: %.1f us" % (p['50.000000']/1000))
print("clat p99: %.1f us" % (p['99.000000']/1000))
print("clat p99.9: %.1f us" % (p['99.900000']/1000))
print("bw_min/max: %d/%d KB/s" % (w['bw_min'], w['bw_max']))