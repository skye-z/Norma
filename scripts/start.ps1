param(
    [string]$Root = ".",
    [string]$ConfigPath = "configs/config.yaml",
    [switch]$RunPreflight = $true,
    [switch]$NoLaunch = $false
)

$ErrorActionPreference = "Stop"

function Write-Step([string]$msg) {
    Write-Host "[STEP] $msg" -ForegroundColor Cyan
}

function Write-Ok([string]$msg) {
    Write-Host "[OK] $msg" -ForegroundColor Green
}

function Write-Warn([string]$msg) {
    Write-Host "[WARN] $msg" -ForegroundColor Yellow
}

function Write-Fail([string]$msg) {
    Write-Host "[FAIL] $msg" -ForegroundColor Red
}

function Get-ConfigValue {
    param(
        [string[]]$Lines,
        [string]$Section,
        [string]$Key
    )

    $inSection = $false
    foreach ($line in $Lines) {
        if ($line -match '^\s*[#]') { continue }
        if ($line -match '^\S') {
            $name = $line.TrimEnd(':').Trim()
            $inSection = ($name -eq $Section)
            continue
        }
        if (-not $inSection) { continue }
        if ($line -match ('^\s+' + [Regex]::Escape($Key) + '\s*:\s*(.+)$')) {
            return $matches[1].Trim().Trim('"').Trim("'")
        }
    }
    return ""
}

function Test-TcpEndpoint {
    param(
        [string]$Endpoint,
        [int]$TimeoutMs = 800
    )

    if (-not ($Endpoint -match '^https?://([^/:]+)(?::(\d+))?')) {
        return $false
    }
    $hostName = $matches[1]
    $port = if ($matches[2]) { [int]$matches[2] } else { 80 }

    $client = New-Object System.Net.Sockets.TcpClient
    try {
        $iar = $client.BeginConnect($hostName, $port, $null, $null)
        if (-not $iar.AsyncWaitHandle.WaitOne($TimeoutMs, $false)) {
            $client.Close()
            return $false
        }
        $client.EndConnect($iar) | Out-Null
        return $true
    }
    catch {
        return $false
    }
    finally {
        $client.Close()
    }
}

function Wait-Endpoint {
    param(
        [string]$Endpoint,
        [int]$TimeoutSec = 20
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        if (Test-TcpEndpoint -Endpoint $Endpoint -TimeoutMs 1000) {
            return $true
        }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

Push-Location $Root
try {
    if (-not (Test-Path $ConfigPath)) {
        Write-Fail "config not found: $ConfigPath"
        exit 1
    }

    if ($RunPreflight) {
        Write-Step "Run preflight checks"
        & powershell -ExecutionPolicy Bypass -File "scripts/preflight.ps1" -Root "." -ConfigPath $ConfigPath
        if ($LASTEXITCODE -ne 0) {
            Write-Fail "preflight failed"
            exit 1
        }
    }

    $cfg = Get-Content -Encoding utf8 $ConfigPath
    $llmProvider = (Get-ConfigValue -Lines $cfg -Section "llm" -Key "provider").ToLowerInvariant()
    $llmEndpoint = Get-ConfigValue -Lines $cfg -Section "llm" -Key "endpoint"
    $ttsProvider = (Get-ConfigValue -Lines $cfg -Section "tts" -Key "provider").ToLowerInvariant()
    $ttsEndpoint = Get-ConfigValue -Lines $cfg -Section "tts" -Key "endpoint"

    Write-Step "Dependency health check"

    if ($llmProvider -eq "ollama") {
        if (-not (Test-TcpEndpoint -Endpoint $llmEndpoint)) {
            Write-Warn "ollama endpoint unreachable, trying to start ollama serve"
            if (Get-Command ollama -ErrorAction SilentlyContinue) {
                Start-Process -FilePath "ollama" -ArgumentList "serve" -WindowStyle Hidden | Out-Null
                if (Wait-Endpoint -Endpoint $llmEndpoint -TimeoutSec 30) {
                    Write-Ok "ollama started: $llmEndpoint"
                } else {
                    Write-Warn "ollama still unreachable: $llmEndpoint"
                }
            } else {
                Write-Warn "ollama command not found; please install/start manually"
            }
        } else {
            Write-Ok "ollama reachable: $llmEndpoint"
        }
    }

    if ($ttsProvider -eq "http" -or $ttsProvider -eq "xtts") {
        if (-not (Test-TcpEndpoint -Endpoint $ttsEndpoint)) {
            $xttsStartCmd = $env:NORMA_XTTS_START_CMD
            if (-not [string]::IsNullOrWhiteSpace($xttsStartCmd)) {
                Write-Warn "tts endpoint unreachable, trying NORMA_XTTS_START_CMD"
                Start-Process -FilePath "powershell" -ArgumentList "-NoProfile", "-Command", $xttsStartCmd -WindowStyle Hidden | Out-Null
                if (Wait-Endpoint -Endpoint $ttsEndpoint -TimeoutSec 30) {
                    Write-Ok "tts started: $ttsEndpoint"
                } else {
                    Write-Warn "tts still unreachable: $ttsEndpoint"
                }
            } else {
                Write-Warn "tts endpoint unreachable and NORMA_XTTS_START_CMD not set"
            }
        } else {
            Write-Ok "tts reachable: $ttsEndpoint"
        }
    }

    Write-Step "Startup self-check summary"
    Write-Host "  llm.provider=$llmProvider"
    Write-Host "  llm.endpoint=$llmEndpoint"
    Write-Host "  tts.provider=$ttsProvider"
    Write-Host "  tts.endpoint=$ttsEndpoint"

    if ($NoLaunch) {
        Write-Ok "NoLaunch enabled, skip app launch"
        return
    }

    Write-Step "Launch Norma"
    if (Test-Path "norma.exe") {
        Start-Process -FilePath ".\norma.exe" | Out-Null
        Write-Ok "launched .\\norma.exe"
    } else {
        $env:GOCACHE = Join-Path (Get-Location) '.gocache'
        $env:GOMODCACHE = Join-Path (Get-Location) '.gomodcache'
        Start-Process -FilePath "go" -ArgumentList "run", "./cmd/norma" -WindowStyle Normal | Out-Null
        Write-Ok "launched go run ./cmd/norma"
    }
}
finally {
    Pop-Location
}