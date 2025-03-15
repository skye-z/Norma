param(
    [string]$Root = ".",
    [switch]$WhatIf
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$projectRoot = (Resolve-Path $Root).Path
$targets = @(
    ".gocache",
    ".gomodcache",
    "dist",
    "diagnostics",
    "logs"
)

Write-Host ("[clean] root={0}" -f $projectRoot)
foreach ($target in $targets) {
    $path = Join-Path $projectRoot $target
    if (-not (Test-Path $path)) {
        Write-Host ("[skip] missing {0}" -f $target)
        continue
    }
    if ($WhatIf) {
        Write-Host ("[whatif] remove {0}" -f $target)
        continue
    }
    Remove-Item -Recurse -Force $path
    Write-Host ("[ok] removed {0}" -f $target)
}