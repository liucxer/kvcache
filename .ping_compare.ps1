# 对比测试：本机 ICMP ping 128.12 vs 通过 proxy exec ping loopback vs proxy 纯调用开销
$ErrorActionPreference = "Continue"
$n = ".\nefsproxy.exe"
$proxyAddr = "100.71.128.12:9527"
$count = 20

Write-Host "=== 1. 本机 ICMP ping 100.71.128.12 (n=$count) ===" -ForegroundColor Cyan
$ping1 = Test-Connection -ComputerName 100.71.128.12 -Count $count -ErrorAction SilentlyContinue
$lat1 = $ping1 | Where-Object { $_.Status -eq "Success" } | ForEach-Object { $_.ResponseTime }
if ($lat1.Count -gt 0) {
    $avg1 = ($lat1 | Measure-Object -Average).Average
    $min1 = ($lat1 | Measure-Object -Minimum).Minimum
    $max1 = ($lat1 | Measure-Object -Maximum).Maximum
    $drop1 = $count - $lat1.Count
    Write-Host "  success=$($lat1.Count) drop=$drop1 min=$min1ms avg=$([math]::Round($avg1,1))ms max=$max1ms"
} else {
    Write-Host "  ALL DROPPED"
}

Write-Host "`n=== 2. proxy exec 在 128.12 本机 ping 127.0.0.1 (loopback, c=$count) ===" -ForegroundColor Cyan
$sw = [System.Diagnostics.Stopwatch]::StartNew()
$res = & $n exec "ping -c $count 127.0.0.1" 2>&1
$sw.Stop()
$proxyCallMs = $sw.Elapsed.TotalMilliseconds
Write-Host "  [proxy 调用端到端总耗时: $([math]::Round($proxyCallMs,1))ms]"
$ping2out = ($res | Out-String)
# 解析 linux ping 输出
$matches2 = [regex]::Matches($ping2out, "time=([\d.]+) ms")
$lat2 = @()
foreach ($m in $matches2) { $lat2 += [double]$m.Groups[1].Value }
if ($lat2.Count -gt 0) {
    $avg2 = ($lat2 | Measure-Object -Average).Average
    $min2 = ($lat2 | Measure-Object -Minimum).Minimum
    $max2 = ($lat2 | Measure-Object -Maximum).Maximum
    Write-Host "  loopback ping: success=$($lat2.Count) min=$min2 ms avg=$([math]::Round($avg2,2)) ms max=$max2 ms"
    Write-Host "  (服务端 loopback ICMP 几乎为 0，主要看 proxy 调用开销)"
}

Write-Host "`n=== 3. proxy 单次纯调用开销 (exec echo x 10) ===" -ForegroundColor Cyan
$pure = @()
for ($i=1; $i -le 10; $i++) {
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    & $n exec "echo ok" 2>$null | Out-Null
    $sw.Stop()
    $pure += $sw.Elapsed.TotalMilliseconds
}
$avgP = ($pure | Measure-Object -Average).Average
$minP = ($pure | Measure-Object -Minimum).Minimum
$maxP = ($pure | Measure-Object -Maximum).Maximum
Write-Host "  pure proxy call: min=$([math]::Round($minP,1))ms avg=$([math]::Round($avgP,1))ms max=$([math]::Round($maxP,1))ms"

Write-Host "`n=== 4. proxy exec 内执行 ping 128.12 自身 IP (跨网卡, c=$count) ===" -ForegroundColor Cyan
# 服务端 ping 自己的网卡 IP（非 loopback），更接近真实跨跳
$sw = [System.Diagnostics.Stopwatch]::StartNew()
$res = & $n exec "ping -c $count 100.71.128.12" 2>&1
$sw.Stop()
$totalMs = $sw.Elapsed.TotalMilliseconds
$pingOut = ($res | Out-String)
Write-Host "  [proxy 调用端到端总耗时: $([math]::Round($totalMs,1))ms]"
$matches4 = [regex]::Matches($pingOut, "time=([\d.]+) ms")
$lat4 = @()
foreach ($m in $matches4) { $lat4 += [double]$m.Groups[1].Value }
if ($lat4.Count -gt 0) {
    $avg4 = ($lat4 | Measure-Object -Average).Average
    $min4 = ($lat4 | Measure-Object -Minimum).Minimum
    $max4 = ($lat4 | Measure-Object -Maximum).Maximum
    Write-Host "  server-side ping 128.12: success=$($lat4.Count) min=$min4 ms avg=$([math]::Round($avg4,2)) ms max=$max4 ms"
}
Write-Host "  server ping 总耗时(ms): $([math]::Round(($lat4 | Measure-Object -Sum).Sum,0))"
Write-Host "  proxy 框架额外开销(总耗时 - server ping 耗时): $([math]::Round($totalMs - ($lat4 | Measure-Object -Sum).Sum,1))ms"

Write-Host "`n=== 对比总结 ===" -ForegroundColor Green
Write-Host ("{0,-40} {1,15}" -f "路径", "avg latency")
Write-Host ("{0,-40} {1,12}ms" -f "本机 ICMP -> 128.12 (跨网)", [math]::Round($avg1,1))
Write-Host ("{0,-40} {1,12}ms" -f "proxy 纯调用开销 (echo)", [math]::Round($avgP,1))
if ($lat4.Count -gt 0) {
    Write-Host ("{0,-40} {1,12}ms" -f "proxy exec 内 ICMP->128.12", [math]::Round($avg4,2))
}
