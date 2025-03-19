param(
    [string]$Root = ".",
    [string]$ConfigPath = "config.yaml",
    [switch]$RunPreflight = $true,
    [switch]$NoLaunch = $false
)

$ErrorActionPreference = "Stop"

$report = [ordered]@{
    timestamp_utc = (Get-Date).ToUniversalTime().ToString("o")
    root = $Root
    config_path = $ConfigPath
    preflight = "unknown"
    llm_provider = ""
    llm_endpoint = ""
    llm_reachable = $false
    tts_provider = ""
    tts_endpoint = ""
    tts_reachable = $false
    launch_attempted = $false
    launch_mode = "none"
    warnings = @()
}

function Add-ReportWarning([string]$msg) {
    $report.warnings = @($report.warnings) + $msg
}

function Write-Step([string]$msg) {
    Write-Host "[STEP] $msg" -ForegroundColor Cyan
}

function Write-Ok([string]$msg) {
    Write-Host "[OK] $msg" -ForegroundColor Green
}

function Write-Warn([string]$msg) {
    Add-ReportWarning $msg
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


function Parse-Endpoint {
    param([string]$Endpoint)
    if ($Endpoint -notmatch '^https?://([^/:]+)(?::(\d+))?') { return $null }
    return [pscustomobject]@{
        Host = $matches[1]
        Port = $(if ($matches[2]) { [int]$matches[2] } else { 80 })
    }
}

function Find-FreePort {
    param(
        [int]$StartPort,
        [int]$EndPort,
        [string]$Host = "127.0.0.1"
    )

    for ($p = $StartPort; $p -le $EndPort; $p++) {
        $listener = $null
        try {
            $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Parse($Host), $p)
            $listener.Start()
            return $p
        }
        catch {
            # continue
        }
        finally {
            if ($null -ne $listener) {
                $listener.Stop()
            }
        }
    }
    return $null
}
function Write-StartupReport {
    $logDir = Join-Path (Get-Location) "resource/logs"
    if (-not (Test-Path $logDir)) {
        New-Item -ItemType Directory -Force $logDir | Out-Null
    }
    $reportPath = Join-Path $logDir "startup_report.json"
    $json = $report | ConvertTo-Json -Depth 4
    [System.IO.File]::WriteAllText($reportPath, $json, [System.Text.UTF8Encoding]::new($false))
    Write-Ok "startup report written: resource/logs/startup_report.json"
}

Push-Location $Root
try {
    if (-not (Test-Path $ConfigPath)) {
        Write-Fail "config not found: $ConfigPath"
        $report.preflight = "config_missing"
        Write-StartupReport
        exit 1
    }

    if ($RunPreflight) {
        Write-Step "Run preflight checks"
        & powershell -ExecutionPolicy Bypass -File "scripts/preflight.ps1" -Root "." -ConfigPath $ConfigPath
        if ($LASTEXITCODE -ne 0) {
            Write-Fail "preflight failed"
            $report.preflight = "failed"
            Write-StartupReport
            exit 1
        }
        $report.preflight = "passed"
    } else {
        $report.preflight = "skipped"
    }

    $cfg = Get-Content -Encoding utf8 $ConfigPath
    $providersRoot = Get-ConfigValue -Lines $cfg -Section "providers" -Key "root"
    if ([string]::IsNullOrWhiteSpace($providersRoot)) { $providersRoot = "resource/runtime/providers" }
    $providersRoot = Join-Path (Get-Location) $providersRoot
    $llmProvider = (Get-ConfigValue -Lines $cfg -Section "llm" -Key "provider").ToLowerInvariant()
    $llmEndpoint = Get-ConfigValue -Lines $cfg -Section "llm" -Key "endpoint"
    $ttsProvider = (Get-ConfigValue -Lines $cfg -Section "tts" -Key "provider").ToLowerInvariant()
    $ttsEndpoint = Get-ConfigValue -Lines $cfg -Section "tts" -Key "endpoint"

    $report.llm_provider = $llmProvider
    $report.llm_endpoint = $llmEndpoint
    $report.tts_provider = $ttsProvider
    $report.tts_endpoint = $ttsEndpoint
    $llmLaunchEndpoint = $llmEndpoint
    $ttsLaunchEndpoint = $ttsEndpoint

    Write-Step "Dependency health check"

    if ($llmProvider -eq "ollama") {
        $ollamaStartCmd = $env:NORMA_OLLAMA_START_CMD
        if ([string]::IsNullOrWhiteSpace($ollamaStartCmd)) {
            $managedOllama = Join-Path $providersRoot "llm/current/ollama.exe"
            if (-not (Test-Path $managedOllama)) {
                $managedOllama = Join-Path $providersRoot "llm/current/ollama"
            }
            if (Test-Path $managedOllama) {
                $ollamaStartCmd = "& '$managedOllama' serve"
            } elseif (Get-Command ollama -ErrorAction SilentlyContinue) {
                $ollamaStartCmd = "ollama serve"
            }
        }

        if (-not (Test-TcpEndpoint -Endpoint $llmEndpoint)) {
            Write-Warn "ollama endpoint unreachable, trying start command"
            if (-not [string]::IsNullOrWhiteSpace($ollamaStartCmd)) {
                Start-Process -FilePath "powershell" -ArgumentList "-NoProfile", "-Command", $ollamaStartCmd -WindowStyle Hidden | Out-Null
                if (Wait-Endpoint -Endpoint $llmEndpoint -TimeoutSec 30) {
                    Write-Ok "ollama started: $llmEndpoint"
                } else {
                    Write-Warn "ollama still unreachable: $llmEndpoint"
                    $ep = Parse-Endpoint -Endpoint $llmEndpoint
                    if ($null -ne $ep) {
                        $alt = Find-FreePort -StartPort ($ep.Port + 1) -EndPort ($ep.Port + 20)
                        if ($null -ne $alt) {
                            $llmLaunchEndpoint = "http://$($ep.Host):$alt"
                            if ($ollamaStartCmd.Contains("{port}")) {
                                $altCmd = $ollamaStartCmd.Replace("{port}", [string]$alt)
                            } else {
                                $altCmd = "`$env:OLLAMA_HOST='http://$($ep.Host):$alt'; ollama serve"
                            }
                            Write-Warn "trying alternate ollama port: $llmLaunchEndpoint"
                            Start-Process -FilePath "powershell" -ArgumentList "-NoProfile", "-Command", $altCmd -WindowStyle Hidden | Out-Null
                            if (Wait-Endpoint -Endpoint $llmLaunchEndpoint -TimeoutSec 30) {
                                Write-Ok "ollama started on alternate port: $llmLaunchEndpoint"
                            } else {
                                Write-Warn "alternate ollama port still unreachable: $llmLaunchEndpoint"
                                $llmLaunchEndpoint = $llmEndpoint
                            }
                        }
                    }
                }
            } else {
                Write-Warn "ollama command not found; please install/start manually"
            }
        } else {
            Write-Ok "ollama reachable: $llmEndpoint"
        }
        $report.llm_reachable = (Test-TcpEndpoint -Endpoint $llmLaunchEndpoint)
        $report.llm_endpoint = $llmLaunchEndpoint
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

                    # Optional fallback: command template with {port}, choose alternative free port.
                    if ($xttsStartCmd.Contains("{port}")) {
                        $ep = Parse-Endpoint -Endpoint $ttsEndpoint
                        if ($null -ne $ep) {
                            $alt = Find-FreePort -StartPort ($ep.Port + 1) -EndPort ($ep.Port + 20)
                            if ($null -ne $alt) {
                                $cmd = $xttsStartCmd.Replace("{port}", [string]$alt)
                                $ttsLaunchEndpoint = "http://$($ep.Host):$alt"
                                Write-Warn "trying alternate tts port: $ttsLaunchEndpoint"
                                Start-Process -FilePath "powershell" -ArgumentList "-NoProfile", "-Command", $cmd -WindowStyle Hidden | Out-Null
                                if (Wait-Endpoint -Endpoint $ttsLaunchEndpoint -TimeoutSec 30) {
                                    Write-Ok "tts started on alternate port: $ttsLaunchEndpoint"
                                } else {
                                    Write-Warn "alternate tts port still unreachable: $ttsLaunchEndpoint"
                                    $ttsLaunchEndpoint = $ttsEndpoint
                                }
                            }
                        }
                    }
                }
            } else {
                Write-Warn "tts endpoint unreachable and NORMA_XTTS_START_CMD not set"
            }
        } else {
            Write-Ok "tts reachable: $ttsEndpoint"
        }
        $report.tts_reachable = (Test-TcpEndpoint -Endpoint $ttsLaunchEndpoint)
        $report.tts_endpoint = $ttsLaunchEndpoint
    }

    Write-Step "Startup self-check summary"
    Write-Host "  llm.provider=$llmProvider"
    Write-Host "  llm.endpoint=$llmLaunchEndpoint"
    Write-Host "  tts.provider=$ttsProvider"
    Write-Host "  tts.endpoint=$ttsLaunchEndpoint"

    if ($env:NORMA_ENABLE_WATCHDOG -eq "1") {
        Write-Step "Start dependency watchdog in background"
        Start-Process -FilePath "powershell" -ArgumentList "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "scripts/watchdog.ps1", "-Root", ".", "-ConfigPath", $ConfigPath -WindowStyle Hidden | Out-Null
        Write-Ok "watchdog started"
    }

    if ($llmLaunchEndpoint -ne $llmEndpoint) {
        $env:NORMA_LLM_ENDPOINT = $llmLaunchEndpoint
        Write-Warn "apply runtime llm endpoint override: NORMA_LLM_ENDPOINT=$llmLaunchEndpoint"
    }

    if ($ttsLaunchEndpoint -ne $ttsEndpoint) {
        $env:NORMA_TTS_ENDPOINT = $ttsLaunchEndpoint
        Write-Warn "apply runtime tts endpoint override: NORMA_TTS_ENDPOINT=$ttsLaunchEndpoint"
    }

    if ($NoLaunch) {
        Write-Ok "NoLaunch enabled, skip app launch"
        Write-StartupReport
        return
    }

    Write-Step "Launch Norma"
    $report.launch_attempted = $true
    if (Test-Path "Norma.exe") {
        Start-Process -FilePath ".\\Norma.exe" | Out-Null
        $report.launch_mode = "binary"
        Write-Ok "launched .\\Norma.exe"
    } else {
        $env:GOCACHE = Join-Path (Get-Location) '.gocache'
        $env:GOMODCACHE = Join-Path (Get-Location) '.gomodcache'
        $cmd = "$env:GOCACHE=(Join-Path (Get-Location) '.gocache'); $env:GOMODCACHE=(Join-Path (Get-Location) '.gomodcache'); wails dev"
        Start-Process -FilePath "powershell" -ArgumentList "-NoProfile", "-Command", $cmd -WindowStyle Normal | Out-Null
        $report.launch_mode = "wails_dev"
        Write-Ok "launched wails dev"
    }

    Write-StartupReport
}
finally {
    Pop-Location
}
