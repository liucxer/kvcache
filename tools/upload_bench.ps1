# 分块上传 bench_linux_arm64 到编译机 /tmp/bench_new
$file = "d:\workspace\kvcache\bench_linux_arm64"
$b64 = [Convert]::ToBase64String([IO.File]::ReadAllBytes($file))
$chunkSize = 32000  # 每块字符数
$totalChunks = [math]::Ceiling($b64.Length / $chunkSize)
Write-Output "Total base64: $($b64.Length) chars, $totalChunks chunks"

# 先清空远程文件
go run ./tools/ssh_helper run "rm -f /tmp/bench_new.b64"

for ($i = 0; $i -lt $totalChunks; $i++) {
    $chunk = $b64.Substring($i * $chunkSize, [math]::Min($chunkSize, $b64.Length - $i * $chunkSize))
    $cmd = "printf '%s' '$chunk' >> /tmp/bench_new.b64"
    go run ./tools/ssh_helper run $cmd | Out-Null
    $pct = [math]::Round(($i + 1) / $totalChunks * 100)
    Write-Output "Chunk $($i+1)/$totalChunks ($pct%)"
}

# 解码并传到 128.12
go run ./tools/ssh_helper run "base64 -d /tmp/bench_new.b64 > /tmp/bench_new && chmod +x /tmp/bench_new && ls -la /tmp/bench_new && scp -o StrictHostKeyChecking=no /tmp/bench_new 100.71.128.12:/root/bench_new && echo UPLOAD_OK"