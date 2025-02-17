param(
    [string]$OutDir = "dist"
)

$ErrorActionPreference = "Stop"
$fails = New-Object System.Collections.Generic.List[string]

function Fail([string]$msg) {
    $fails.Add($msg)
    Write-Host "[FAIL] $msg" -ForegroundColor Red
}

function Ok([string]$msg) {
    Write-Host "[OK] $msg" -ForegroundColor Green
}

if (-not (Test-Path $OutDir)) {
    Fail "output dir missing: $OutDir"
} else {
    Ok "output dir exists: $OutDir"
}

$requiredBins = @(
    "norma-windows-amd64.exe",
    "norma-linux-amd64",
    "norma-darwin-amd64"
)

foreach ($b in $requiredBins) {
    $p = Join-Path $OutDir $b
    if (Test-Path $p) {
        Ok "binary exists: $b"
    } else {
        Fail "binary missing: $b"
    }
}

if (Test-Path "configs/config.yaml") {
    Ok "config exists: configs/config.yaml"
} else {
    Write-Host "[WARN] config missing in workspace: configs/config.yaml" -ForegroundColor Yellow
}

if (Test-Path "model") {
    Ok "model dir exists: model"
} else {
    Write-Host "[WARN] model dir missing in workspace: model" -ForegroundColor Yellow
}

if ($fails.Count -gt 0) {
    exit 1
}

Write-Host "[OK] release layout check passed"