param(
    [string]$OutDir = "dist"
)

$ErrorActionPreference = "Stop"
$fails = New-Object System.Collections.Generic.List[string]
function Fail([string]$msg) { $fails.Add($msg); Write-Host "[FAIL] $msg" -ForegroundColor Red }
function Ok([string]$msg) { Write-Host "[OK] $msg" -ForegroundColor Green }
if (-not (Test-Path $OutDir)) { Fail "output dir missing: $OutDir" } else { Ok "output dir exists: $OutDir" }
foreach ($pkg in @('Norma-windows-amd64','Norma-linux-amd64','Norma-darwin-amd64')) {
    $root = Join-Path $OutDir $pkg
    if (-not (Test-Path $root)) { Fail "package missing: $pkg"; continue }
    Ok "package exists: $pkg"
    $binary = if ($pkg -like 'Norma-windows-*') { 'Norma.exe' } else { 'Norma' }
    foreach ($path in @($binary,'config.example.yaml','config.yaml','debug','resource/model','resource/data','resource/logs','resource/diagnostics','resource/runtime/providers')) {
        $full = Join-Path $root $path
        if (Test-Path $full) { Ok "$pkg => $path" } else { Fail "$pkg missing: $path" }
    }
}
foreach ($f in @('release_manifest.json','SHA256SUMS.txt','INSTALL.md')) { if (Test-Path (Join-Path $OutDir $f)) { Ok "release artifact exists: $f" } else { Fail "release artifact missing: $f" } }
if ($fails.Count -gt 0) { exit 1 }
Write-Host "[OK] release layout check passed"
