param(
  [string]$OutDir = "dist",
  [string]$Version = "dev"
)

$ErrorActionPreference = "Stop"
$packages = Get-ChildItem -Path $OutDir -Directory | Sort-Object Name
$manifest = [ordered]@{
  version = $Version
  generated_at_utc = (Get-Date).ToUniversalTime().ToString("o")
  packages = @()
}
$checksums = New-Object System.Collections.Generic.List[string]
foreach ($pkg in $packages) {
  $files = Get-ChildItem -Path $pkg.FullName -File -Recurse | Sort-Object FullName
  $entries = @()
  foreach ($file in $files) {
    $relative = $file.FullName.Substring($pkg.FullName.Length + 1).Replace('\','/')
    $hash = (Get-FileHash -Path $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    $checksums.Add("$hash  $($pkg.Name)/$relative")
    $entries += [ordered]@{ path = $relative; size = $file.Length; sha256 = $hash }
  }
  $manifest.packages += [ordered]@{ name = $pkg.Name; files = $entries }
}
[System.IO.File]::WriteAllLines((Join-Path $OutDir 'SHA256SUMS.txt'), $checksums, [System.Text.UTF8Encoding]::new($false))
[System.IO.File]::WriteAllText((Join-Path $OutDir 'release_manifest.json'), ($manifest | ConvertTo-Json -Depth 6), [System.Text.UTF8Encoding]::new($false))
  $install = @(
    '# Norma Install Guide',
    '',
    'Each package directory uses this layout:',
    '- `Norma.exe` or `Norma`',
    '- `config.example.yaml`',
    '- `config.yaml`',
    '- `console/`',
    '- `resource/model/`',
    '- `resource/data/`',
    '- `resource/logs/`',
    '- `resource/diagnostics/`',
    '- `resource/runtime/providers/`',
    '- `resource/runtime/providers/tts/current/model.onnx` (Windows embedded)',
    '',
    'On first run, edit `config.yaml` if needed, then start Norma.'
  ) -join "`n"
[System.IO.File]::WriteAllText((Join-Path $OutDir 'INSTALL.md'), $install, [System.Text.UTF8Encoding]::new($false))
Write-Host "[OK] release manifest written"
