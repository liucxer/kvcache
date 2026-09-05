# 两种访问方式效率对比测试
$results = @()

# === 1. 简单命令执行：hostname，各5次 ===
Write-Output "=== 1. 简单命令执行 hostname (5次平均) ==="

# ssh_helper
$sshTimes = @()
for ($i = 1; $i -le 5; $i++) {
    $t = Measure-Command {
        go run ./tools/ssh_helper jump 100.71.128.12 "hostname" 2>&1 | Out-Null
    }
    $sshTimes += $t.TotalMilliseconds
    Write-Output "  ssh_helper run ${i}: $([math]::Round($t.TotalMilliseconds))ms"
}
$sshAvg = ($sshTimes | Measure-Command -Average).Average
# 修正：用 Measure-Object
$sshAvg = ($sshTimes | Measure-Object -Average).Average
Write-Output "  ssh_helper AVG: $([math]::Round($sshAvg))ms"

# nefs-proxy
$proxyTimes = @()
for ($i = 1; $i -le 5; $i++) {
    $t = Measure-Command {
        py d:\workspace\kvcache\nefs-proxy\proxy_client.py --node 128 exec --cmd "hostname" 2>&1 | Out-Null
    }
    $proxyTimes += $t.TotalMilliseconds
    Write-Output "  nefs-proxy run ${i}: $([math]::Round($t.TotalMilliseconds))ms"
}
$proxyAvg = ($proxyTimes | Measure-Object -Average).Average
Write-Output "  nefs-proxy AVG: $([math]::Round($proxyAvg))ms"

$results += [PSCustomObject]@{
    Test = "简单命令 hostname"
    ssh_helper_ms = [math]::Round($sshAvg)
    nefs_proxy_ms = [math]::Round($proxyAvg)
    faster = if ($sshAvg -lt $proxyAvg) { "ssh_helper" } else { "nefs-proxy" }
    speedup = [math]::Round([math]::Max($sshAvg,$proxyAvg) / [math]::Min($sshAvg,$proxyAvg), 2)
}

# === 2. 文件上传 10MB ===
Write-Output ""
Write-Output "=== 2. 上传 10MB 文件 (3次平均) ==="

# 生成测试文件
$testFile = "d:\workspace\kvcache\test_10mb.bin"
$bytes = New-Object byte[] 10485760
(New-Object Random).NextBytes($bytes)
[IO.File]::WriteAllBytes($testFile, $bytes)
Write-Output "测试文件: $((Get-Item $testFile).Length / 1MB) MB"

# ssh_helper 上传（通过编译机 scp）
$sshUploadTimes = @()
for ($i = 1; $i -le 3; $i++) {
    $t = Measure-Command {
        $b = [Convert]::ToBase64String([IO.File]::ReadAllBytes($testFile))
        go run ./tools/ssh_helper run "echo $b | base64 -d > /tmp/test_10mb.bin && echo OK" 2>&1 | Out-Null
    }
    $sshUploadTimes += $t.TotalMilliseconds
    Write-Output "  ssh_helper upload run ${i}: $([math]::Round($t.TotalMilliseconds))ms"
}
$sshUploadAvg = ($sshUploadTimes | Measure-Object -Average).Average
Write-Output "  ssh_helper UPLOAD AVG: $([math]::Round($sshUploadAvg))ms"

# nefs-proxy 上传
$proxyUploadTimes = @()
for ($i = 1; $i -le 3; $i++) {
    $t = Measure-Command {
        py d:\workspace\kvcache\nefs-proxy\proxy_client.py --node 128 upload --local $testFile --remote /tmp/test_10mb_proxy.bin 2>&1 | Out-Null
    }
    $proxyUploadTimes += $t.TotalMilliseconds
    Write-Output "  nefs-proxy upload run ${i}: $([math]::Round($t.TotalMilliseconds))ms"
}
$proxyUploadAvg = ($proxyUploadTimes | Measure-Object -Average).Average
Write-Output "  nefs-proxy UPLOAD AVG: $([math]::Round($proxyUploadAvg))ms"

$results += [PSCustomObject]@{
    Test = "上传 10MB"
    ssh_helper_ms = [math]::Round($sshUploadAvg)
    nefs_proxy_ms = [math]::Round($proxyUploadAvg)
    faster = if ($sshUploadAvg -lt $proxyUploadAvg) { "ssh_helper" } else { "nefs-proxy" }
    speedup = [math]::Round([math]::Max($sshUploadAvg,$proxyUploadAvg) / [math]::Min($sshUploadAvg,$proxyUploadAvg), 2)
}

# === 3. 文件下载 10MB ===
Write-Output ""
Write-Output "=== 3. 下载 10MB 文件 (3次平均) ==="

# ssh_helper 下载
$sshDlTimes = @()
for ($i = 1; $i -le 3; $i++) {
    $t = Measure-Command {
        go run ./tools/ssh_helper run "base64 -w0 /tmp/test_10mb.bin" 2>&1 | Out-Null
    }
    $sshDlTimes += $t.TotalMilliseconds
    Write-Output "  ssh_helper download run ${i}: $([math]::Round($t.TotalMilliseconds))ms"
}
$sshDlAvg = ($sshDlTimes | Measure-Object -Average).Average
Write-Output "  ssh_helper DOWNLOAD AVG: $([math]::Round($sshDlAvg))ms"

# nefs-proxy 下载
$proxyDlTimes = @()
for ($i = 1; $i -le 3; $i++) {
    $t = Measure-Command {
        py d:\workspace\kvcache\nefs-proxy\proxy_client.py --node 128 download --remote /tmp/test_10mb_proxy.bin --local "$testFile.dl" 2>&1 | Out-Null
    }
    $proxyDlTimes += $t.TotalMilliseconds
    Write-Output "  nefs-proxy download run ${i}: $([math]::Round($t.TotalMilliseconds))ms"
}
$proxyDlAvg = ($proxyDlTimes | Measure-Object -Average).Average
Write-Output "  nefs-proxy DOWNLOAD AVG: $([math]::Round($proxyDlAvg))ms"

$results += [PSCustomObject]@{
    Test = "下载 10MB"
    ssh_helper_ms = [math]::Round($sshDlAvg)
    nefs_proxy_ms = [math]::Round($proxyDlAvg)
    faster = if ($sshDlAvg -lt $proxyDlAvg) { "ssh_helper" } else { "nefs-proxy" }
    speedup = [math]::Round([math]::Max($sshDlAvg,$proxyDlAvg) / [math]::Min($sshDlAvg,$proxyDlAvg), 2)
}

# === 4. 汇总 ===
Write-Output ""
Write-Output "========== 效率对比汇总 =========="
$results | Format-Table -AutoSize

# 清理
Remove-Item $testFile -ErrorAction SilentlyContinue
Remove-Item "$testFile.dl" -ErrorAction SilentlyContinue