param(
  [string]$OutDir = "dist",
  [string]$Version = "dev"
)

$ErrorActionPreference = "Stop"
$items = Get-ChildItem -Path $OutDir -File | Sort-Object Name
$checksums = New-Object System.Collections.Generic.List[string]
$manifest = [ordered]@{
  version = $Version
  generated_at_utc = (Get-Date).ToUniversalTime().ToString("o")
  artifacts = @()
}
foreach ($item in $items) {
  $hash = (Get-FileHash -Path $item.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
  $checksums.Add("$hash  $($item.Name)")
  $manifest.artifacts += [ordered]@{ name = $item.Name; size = $item.Length; sha256 = $hash }
}
[System.IO.File]::WriteAllLines((Join-Path $OutDir 'SHA256SUMS.txt'), $checksums, [System.Text.UTF8Encoding]::new($false))
[System.IO.File]::WriteAllText((Join-Path $OutDir 'release_manifest.json'), ($manifest | ConvertTo-Json -Depth 5), [System.Text.UTF8Encoding]::new($false))
$install = @(
  '# Norma Install Guide',
  '',
  '## Artifacts',
  '',
  'Place the selected binary together with:',
  '- `configs/config.yaml`',
  '- `model/`',
  '- local runtime dependencies if using Whisper.cpp / XTTS / Ollama',
  '',
  '## First Run',
  '',
  '```powershell',
  'powershell -ExecutionPolicy Bypass -File scripts/start.ps1 -Root .',
  '```',
  '',
  '## Verification',
  '',
  '- Run `scripts/preflight.ps1`',
  '- Verify `SHA256SUMS.txt`',
  '- Check `release_manifest.json`'
) -join "`n"
[System.IO.File]::WriteAllText((Join-Path $OutDir 'INSTALL.md'), $install, [System.Text.UTF8Encoding]::new($false))
if ($env:NORMA_SIGN_FILE_CMD) {
  $signed = New-Object System.Collections.Generic.List[string]
  foreach ($item in $items) {
    $cmd = $env:NORMA_SIGN_FILE_CMD.Replace('{file}', '"' + $item.FullName + '"')
    powershell -NoProfile -Command $cmd
    if ($LASTEXITCODE -eq 0) { $signed.Add($item.Name) }
  }
  [System.IO.File]::WriteAllLines((Join-Path $OutDir 'SIGNATURES.txt'), $signed, [System.Text.UTF8Encoding]::new($false))
}
Write-Host "[OK] release manifest written"