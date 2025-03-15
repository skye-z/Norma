param(
    [string]$Root = ".",
    [string]$ConfigPath = "config.yaml",
    [string]$OutDir = "diagnostics"
)

$ErrorActionPreference = "Stop"

function Mask-Line([string]$line) {
    $patterns = @(
        '(?i)(api[_-]?key\s*:\s*)(.+)$',
        '(?i)(token\s*:\s*)(.+)$',
        '(?i)(password\s*:\s*)(.+)$',
        '(?i)(secret\s*:\s*)(.+)$'
    )
    $masked = $line
    foreach ($p in $patterns) {
        $masked = [regex]::Replace($masked, $p, '$1***')
    }
    return $masked
}

Push-Location $Root
try {
    $ts = Get-Date -Format "yyyyMMdd-HHmmss"
    $tmp = Join-Path $OutDir ("diag-" + $ts)
    $pkg = Join-Path $OutDir ("diag-" + $ts + ".zip")

    New-Item -ItemType Directory -Force $tmp | Out-Null
    New-Item -ItemType Directory -Force (Join-Path $tmp "logs") | Out-Null

    # 1) Config summary (masked)
    if (Test-Path $ConfigPath) {
        $cfgLines = Get-Content -Encoding utf8 $ConfigPath | ForEach-Object { Mask-Line $_ }
        [System.IO.File]::WriteAllLines((Join-Path $tmp "config.sanitized.yaml"), $cfgLines, [System.Text.UTF8Encoding]::new($false))
    }

    # 2) Key logs
    $logFiles = @(
        "resource/logs/app.log",
        "resource/logs/audio.log",
        "resource/logs/rag.log",
        "logs/startup_report.json"
    )
    foreach ($lf in $logFiles) {
        if (Test-Path $lf) {
            Copy-Item $lf (Join-Path $tmp "logs") -Force
        }
    }

    # 3) Environment report
    $envReport = [ordered]@{
        timestamp_utc = (Get-Date).ToUniversalTime().ToString("o")
        os = [System.Environment]::OSVersion.VersionString
        powershell = $PSVersionTable.PSVersion.ToString()
        go_version = (go version 2>$null)
        git_commit = (git rev-parse --short HEAD 2>$null)
        cwd = (Get-Location).Path
    } | ConvertTo-Json -Depth 3
    [System.IO.File]::WriteAllText((Join-Path $tmp "environment.json"), $envReport, [System.Text.UTF8Encoding]::new($false))

    # 4) Runtime check output
    $preflightOut = Join-Path $tmp "preflight.txt"
    powershell -ExecutionPolicy Bypass -File scripts/preflight.ps1 -Root "." *> $preflightOut

    if (Test-Path $pkg) {
        Remove-Item $pkg -Force
    }
    Compress-Archive -Path (Join-Path $tmp "*") -DestinationPath $pkg -Force

    Write-Host "[OK] diagnostic package: $pkg"
}
finally {
    Pop-Location
}