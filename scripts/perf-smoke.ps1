param(
  [string]$Root = ".",
  [string]$Baseline = "configs/perf-baseline.json"
)
$ErrorActionPreference = "Stop"
if (-not (Test-Path $Baseline)) { throw "baseline missing: $Baseline" }
$baselineObj = Get-Content -Raw -Encoding utf8 $Baseline | ConvertFrom-Json
$logPath = Join-Path $Root 'logs/app.log'
if (-not (Test-Path $logPath)) { throw "log missing: $logPath" }
$text = Get-Content -Raw -Encoding utf8 $logPath
$fails = New-Object System.Collections.Generic.List[string]
if ($text -match 'llm_first_token_ms=(\d+)') { if ([int]$matches[1] -gt [int]$baselineObj.llm_first_token_ms) { $fails.Add("llm_first_token_ms exceeded baseline") } }
if ($text -match 'tts_first_chunk_ms=(\d+)') { if ([int]$matches[1] -gt [int]$baselineObj.tts_first_chunk_ms) { $fails.Add("tts_first_chunk_ms exceeded baseline") } }
if ($fails.Count -gt 0) { $fails | ForEach-Object { Write-Host "[FAIL] $_" -ForegroundColor Red }; exit 1 }
Write-Host '[OK] perf smoke passed'