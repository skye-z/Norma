param(
    [string]$Root = ".",
    [string]$ConfigPath = "config.yaml",
    [string]$ModelDir = "resource/model"
)

$ErrorActionPreference = "Stop"
$failed = New-Object System.Collections.Generic.List[string]
$warnings = New-Object System.Collections.Generic.List[string]

function Add-Fail([string]$msg) {
    $script:failed.Add($msg)
    Write-Host "[FAIL] $msg" -ForegroundColor Red
}

function Add-Warn([string]$msg) {
    $script:warnings.Add($msg)
    Write-Host "[WARN] $msg" -ForegroundColor Yellow
}

function Add-Ok([string]$msg) {
    Write-Host "[OK] $msg" -ForegroundColor Green
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

function Test-Live2DModelDir {
    param(
        [string]$Dir
    )

    $issues = New-Object System.Collections.Generic.List[string]
    $hasCan3 = @(Get-ChildItem -Path $Dir -Filter "*.can3" -File -ErrorAction SilentlyContinue).Count -gt 0
    $hasCmo3 = @(Get-ChildItem -Path $Dir -Filter "*.cmo3" -File -ErrorAction SilentlyContinue).Count -gt 0
    $model3Json = Get-ChildItem -Path $Dir -Filter "*.model3.json" -File -ErrorAction SilentlyContinue | Select-Object -First 1

    if (-not $hasCan3) {
        $issues.Add("missing *.can3")
    }
    if (-not $hasCmo3) {
        $issues.Add("missing *.cmo3")
    }
    if ($null -eq $model3Json) {
        $issues.Add("missing *.model3.json")
    }

    return [PSCustomObject]@{
        ModelDir      = $Dir
        HasCan3       = $hasCan3
        HasCmo3       = $hasCmo3
        HasModel3JSON = ($null -ne $model3Json)
        Issues        = $issues
        OK            = ($issues.Count -eq 0)
    }
}

Push-Location $Root
try {
    # 1) Encoding check (strict)
    & powershell -ExecutionPolicy Bypass -File "scripts/check-utf8.ps1" -Root "." -FailOnBom
    if ($LASTEXITCODE -ne 0) {
        Add-Fail "UTF-8 check failed"
    } else {
        Add-Ok "UTF-8 check passed"
    }

    # 2) Config check
    if (-not (Test-Path $ConfigPath)) {
        Add-Fail "config file missing: $ConfigPath"
    } else {
        Add-Ok "config file found: $ConfigPath"
    }

    $cfgLines = @()
    if (Test-Path $ConfigPath) {
        $cfgLines = Get-Content -Encoding utf8 $ConfigPath
    }

    $required = @(
        @{ section = "database"; key = "path" },
        @{ section = "asr"; key = "provider" },
        @{ section = "llm"; key = "provider" },
        @{ section = "tts"; key = "provider" },
        @{ section = "rag"; key = "top_k" }
    )

    foreach ($item in $required) {
        $v = Get-ConfigValue -Lines $cfgLines -Section $item.section -Key $item.key
        if ([string]::IsNullOrWhiteSpace($v)) {
            Add-Fail "missing config: $($item.section).$($item.key)"
        }
    }

    $asrProvider = (Get-ConfigValue -Lines $cfgLines -Section "asr" -Key "provider").ToLowerInvariant()
    $asrEndpoint = Get-ConfigValue -Lines $cfgLines -Section "asr" -Key "endpoint"
    $llmProvider = (Get-ConfigValue -Lines $cfgLines -Section "llm" -Key "provider").ToLowerInvariant()
    $llmEndpoint = Get-ConfigValue -Lines $cfgLines -Section "llm" -Key "endpoint"
    $ttsProvider = (Get-ConfigValue -Lines $cfgLines -Section "tts" -Key "provider").ToLowerInvariant()
    $ttsEndpoint = Get-ConfigValue -Lines $cfgLines -Section "tts" -Key "endpoint"

    # 3) Model check
    if (-not (Test-Path $ModelDir)) {
        Add-Warn "model dir not found: $ModelDir"
    } else {
        $modelCheck = Test-Live2DModelDir -Dir $ModelDir
        Write-Host "ModelDir: $($modelCheck.ModelDir)"
        Write-Host "HasCan3: $($modelCheck.HasCan3)"
        Write-Host "HasModel3JSON: $($modelCheck.HasModel3JSON)"
        Write-Host "HasCmo3: $($modelCheck.HasCmo3)"
        if (-not $modelCheck.OK) {
            Write-Host "Status: FAILED"
            for ($i = 0; $i -lt $modelCheck.Issues.Count; $i++) {
                Write-Host ("{0}. {1}" -f ($i + 1), $modelCheck.Issues[$i])
            }
            Add-Warn "model check did not pass"
        } else {
            Write-Host "Status: OK"
            Add-Ok "model assets check passed"
        }
    }

    # 4) Port / endpoint checks
    if ($llmProvider -eq "ollama") {
        if ([string]::IsNullOrWhiteSpace($llmEndpoint)) {
            Add-Fail "llm.endpoint is empty while llm.provider=ollama"
        } elseif (-not (Test-TcpEndpoint -Endpoint $llmEndpoint)) {
            Add-Warn "llm endpoint unreachable: $llmEndpoint"
        } else {
            Add-Ok "llm endpoint reachable: $llmEndpoint"
        }
    }

    if ($asrProvider -eq "http") {
        if ([string]::IsNullOrWhiteSpace($asrEndpoint)) {
            Add-Fail "asr.endpoint is empty while asr.provider=http"
        } elseif (-not (Test-TcpEndpoint -Endpoint $asrEndpoint)) {
            Add-Warn "asr endpoint unreachable: $asrEndpoint"
        } else {
            Add-Ok "asr endpoint reachable: $asrEndpoint"
        }
    }

    if ($ttsProvider -eq "http" -or $ttsProvider -eq "xtts") {
        if ([string]::IsNullOrWhiteSpace($ttsEndpoint)) {
            Add-Fail "tts.endpoint is empty while tts.provider=$ttsProvider"
        } elseif (-not (Test-TcpEndpoint -Endpoint $ttsEndpoint)) {
            Add-Warn "tts endpoint unreachable: $ttsEndpoint"
        } else {
            Add-Ok "tts endpoint reachable: $ttsEndpoint"
        }
    }

    # 5) Key metrics structured check (from logs)
    $metricsLog = Join-Path (Get-Location) "resource/logs/app.log"
    if (Test-Path $metricsLog) {
        $tail = Get-Content -Encoding utf8 $metricsLog -Tail 1200
        $summaryLine = $tail | Where-Object { $_ -match '\[metric\] summary' } | Select-Object -Last 1
        $asrLlmLine = $tail | Where-Object { $_ -match '\[metric\] llm_first_token_ms=' } | Select-Object -Last 1
        $asrTtsLine = $tail | Where-Object { $_ -match '\[metric\] tts_first_chunk_ms=' } | Select-Object -Last 1

        if ($null -eq $summaryLine -or $null -eq $asrLlmLine -or $null -eq $asrTtsLine) {
            $hasSession = $tail | Where-Object { $_ -match 'session=' } | Select-Object -First 1
            if ($null -ne $hasSession) {
                Add-Warn "missing structured metric lines in resource/logs/app.log (need llm_first_token_ms, tts_first_chunk_ms, summary)"
            } else {
                Add-Ok "no recent session logs; skip strict metric-line check"
            }
        } else {
            Add-Ok "structured metric lines detected in resource/logs/app.log"
        }
    } else {
        Add-Warn "metrics log not found: resource/logs/app.log"
    }

    # 6) Split-log existence check
    $audioLog = Join-Path (Get-Location) "resource/logs/audio.log"
    $ragLog = Join-Path (Get-Location) "resource/logs/rag.log"
    if (Test-Path $audioLog) { Add-Ok "audio log exists: resource/logs/audio.log" } else { Add-Warn "audio log missing: resource/logs/audio.log" }
    if (Test-Path $ragLog) { Add-Ok "rag log exists: resource/logs/rag.log" } else { Add-Warn "rag log missing: resource/logs/rag.log" }

    Write-Host ""
    Write-Host "Summary: fails=$($failed.Count), warnings=$($warnings.Count)"
    if ($failed.Count -gt 0) {
        exit 1
    }
}
finally {
    Pop-Location
}
