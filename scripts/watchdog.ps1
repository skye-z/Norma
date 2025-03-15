param(
    [string]$Root = ".",
    [string]$ConfigPath = "config.yaml",
    [int]$IntervalSec = 10,
    [switch]$Once = $false
)

$ErrorActionPreference = "Stop"

function Write-Log([string]$msg) {
    Write-Host "[watchdog] $msg"
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

function Heal-Once {
    param([string[]]$cfg)

    $llmProvider = (Get-ConfigValue -Lines $cfg -Section "llm" -Key "provider").ToLowerInvariant()
    $llmEndpoint = Get-ConfigValue -Lines $cfg -Section "llm" -Key "endpoint"
    $ttsProvider = (Get-ConfigValue -Lines $cfg -Section "tts" -Key "provider").ToLowerInvariant()
    $ttsEndpoint = Get-ConfigValue -Lines $cfg -Section "tts" -Key "endpoint"

    if ($llmProvider -eq "ollama") {
        if (-not (Test-TcpEndpoint -Endpoint $llmEndpoint)) {
            Write-Log "llm down: $llmEndpoint, try start ollama serve"
            if (Get-Command ollama -ErrorAction SilentlyContinue) {
                Start-Process -FilePath "ollama" -ArgumentList "serve" -WindowStyle Hidden | Out-Null
            } else {
                Write-Log "ollama command missing"
            }
        }
    }

    if ($ttsProvider -eq "http" -or $ttsProvider -eq "xtts") {
        if (-not (Test-TcpEndpoint -Endpoint $ttsEndpoint)) {
            $cmd = $env:NORMA_XTTS_START_CMD
            if (-not [string]::IsNullOrWhiteSpace($cmd)) {
                Write-Log "tts down: $ttsEndpoint, try NORMA_XTTS_START_CMD"
                Start-Process -FilePath "powershell" -ArgumentList "-NoProfile", "-Command", $cmd -WindowStyle Hidden | Out-Null
            } else {
                Write-Log "tts down: $ttsEndpoint, no NORMA_XTTS_START_CMD configured"
            }
        }
    }
}

Push-Location $Root
try {
    if (-not (Test-Path $ConfigPath)) {
        throw "config not found: $ConfigPath"
    }

    Write-Log "start with config=$ConfigPath interval=${IntervalSec}s once=$Once"
    do {
        $cfg = Get-Content -Encoding utf8 $ConfigPath
        Heal-Once -cfg $cfg

        if ($Once) { break }
        Start-Sleep -Seconds $IntervalSec
    } while ($true)
}
finally {
    Pop-Location
}