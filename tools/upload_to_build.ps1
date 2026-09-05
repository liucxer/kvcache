# 分段上传文件到编译机
$localFile = "d:\workspace\kvcache\kvcache_full.tar.gz"
$remoteFile = "/tmp/kvcache_full.tar.gz"

$bytes = [IO.File]::ReadAllBytes($localFile)
$b64 = [Convert]::ToBase64String($bytes)
$totalLen = $b64.Length
$chunkSize = 60000  # 每段 60KB base64

Write-Output "File: $localFile ($($bytes.Length) bytes, base64=$totalLen chars)"
Write-Output "Chunks: $([math]::Ceiling($totalLen / $chunkSize))"

# 先清空目标文件
$chunk = $b64.Substring(0, [math]::Min($chunkSize, $totalLen))
$cmd = "echo '$chunk' | base64 -d > $remoteFile.part0"
Write-Output "Uploading chunk 1..."
go run ./tools/ssh_helper run $cmd 2>&1 | Out-Null

$offset = $chunkSize
$part = 1
while ($offset -lt $totalLen) {
    $len = [math]::Min($chunkSize, $totalLen - $offset)
    $chunk = $b64.Substring($offset, $len)
    $cmd = "echo '$chunk' | base64 -d >> $remoteFile.part$part"
    Write-Output "Uploading chunk $($part+1)..."
    go run ./tools/ssh_helper run $cmd 2>&1 | Out-Null
    $offset += $chunkSize
    $part++
}

# 合并
Write-Output "Merging $part parts..."
$mergeCmd = "cat $remoteFile.part* > $remoteFile && wc -c $remoteFile && rm -f $remoteFile.part*"
go run ./tools/ssh_helper run $mergeCmd 2>&1