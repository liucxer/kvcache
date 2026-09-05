# 分块上传 kvcache_src.tar.gz
$file = "d:\workspace\kvcache\kvcache_src.tar.gz"
$b64 = [Convert]::ToBase64String([IO.File]::ReadAllBytes($file))
$chunkSize = 16000
$totalChunks = [math]::Ceiling($b64.Length / $chunkSize)
Write-Output "base64 len: $($b64.Length), chunks: $totalChunks"

# 清空远程文件
go run ./tools/ssh_helper run "rm -f /tmp/kvcache_src.tar.gz.b64"

for ($i = 0; $i -lt $totalChunks; $i++) {
    $start = $i * $chunkSize
    $len = [math]::Min($chunkSize, $b64.Length - $start)
    $chunk = $b64.Substring($start, $len)
    $rc = go run ./tools/ssh_helper run "printf '%s' '$chunk' >> /tmp/kvcache_src.tar.gz.b64" 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Output "FAIL chunk $($i+1): $rc"
        exit 1
    }
    if (($i + 1) % 50 -eq 0) {
        Write-Output "Chunk $($i+1)/$totalChunks"
    }
}
Write-Output "All chunks uploaded"

# 解码 + 解压 + 编译
go run ./tools/ssh_helper run "base64 -d /tmp/kvcache_src.tar.gz.b64 > /tmp/kvcache_src.tar.gz && cd /root && rm -rf kvcache_new && mkdir kvcache_new && cd kvcache_new && tar -xzf /tmp/kvcache_src.tar.gz && ls && echo EXTRACT_OK"