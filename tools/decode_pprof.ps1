$content = Get-Content "d:\workspace\kvcache\tools\pprof_all.b64" -Raw
$markers = [regex]::Matches($content, '###MARKER_([\w\.]+?)###\r?\n')
Write-Output "Found $($markers.Count) markers"
foreach ($m in $markers) {
    $name = $m.Groups[1].Value
    $start = $m.Index + $m.Length
    # find next marker or end
    $rest = $content.Substring($start)
    $nextM = [regex]::Match($rest, '###MARKER_')
    $b64len = if ($nextM.Success) { $nextM.Index } else { $rest.Length }
    $b64data = $rest.Substring(0, $b64len).Trim()
    $raw = [Convert]::FromBase64String($b64data)
    $out = "d:\workspace\kvcache\tools\dl_$name"
    [IO.File]::WriteAllBytes($out, $raw)
    Write-Output "$name -> $($raw.Length) bytes"
}