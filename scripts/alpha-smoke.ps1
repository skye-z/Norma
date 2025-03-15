param(
  [string]$Root = ".",
  [string]$ConfigPath = "config.yaml"
)

$ErrorActionPreference = "Stop"
Push-Location $Root
try {
  $env:GOCACHE = Join-Path (Get-Location) '.gocache'
  $env:GOMODCACHE = Join-Path (Get-Location) '.gomodcache'
  if (-not (Test-Path $env:GOCACHE)) { New-Item -ItemType Directory -Path $env:GOCACHE | Out-Null }
  if (-not (Test-Path $env:GOMODCACHE)) { New-Item -ItemType Directory -Path $env:GOMODCACHE | Out-Null }

  $runRoot = Join-Path (Get-Location) 'diagnostics/alpha-smoke'
  $logDir = Join-Path $runRoot 'logs'
  $outputLog = Join-Path $runRoot 'demo-output.log'
  if (Test-Path $runRoot) { Remove-Item -Recurse -Force $runRoot }
  New-Item -ItemType Directory -Path $logDir -Force | Out-Null

  $env:NORMA_APP_LOG_DIR = $logDir
  $env:NORMA_LLM_PROVIDER = 'mock'
  $env:NORMA_ASR_PROVIDER = 'mock'
  $env:NORMA_TTS_PROVIDER = 'mock'
  $env:NORMA_AUDIO_WAKE_MODE = 'vad'

  Write-Host '[alpha] step 1/3: focused voice tests'
  & go test ./internal/audio -run 'TestPipelineUsesFrameASR|TestPipelineSupportsWakeWordMode' -count=1
  if ($LASTEXITCODE -ne 0) { throw 'audio focused tests failed' }
  & go test ./internal/runtime -run 'TestRegressionVoiceQAFlow|TestRegressionVoiceInterrupt' -count=1
  if ($LASTEXITCODE -ne 0) { throw 'runtime focused tests failed' }

  Write-Host '[alpha] step 2/3: demo voice chain run'
  & go run ./cmd/norma -config $ConfigPath -demo *>&1 | Tee-Object -FilePath $outputLog | Out-Host
  if ($LASTEXITCODE -ne 0) { throw 'demo run failed' }

  Write-Host '[alpha] step 3/3: verify logs'
  $appLog = Join-Path $logDir 'app.log'
  if (-not (Test-Path $appLog)) { throw "app log missing: $appLog" }
  $text = Get-Content -Raw -Encoding utf8 $appLog
  $required = @(
    '[demo] asr.partial=',
    '[runtime] asr.final -> Thinking',
    '[demo] llm.token=',
    '[demo] tts.chunk bytes=',
    '[metric] summary',
    '[demo] final state=Listening'
  )
  foreach ($needle in $required) {
    if (-not $text.Contains($needle)) {
      throw "missing expected log fragment: $needle"
    }
  }

  Write-Host '[OK] v0.3 Alpha acceptance passed' -ForegroundColor Green
  Write-Host "[OK] logs: $appLog" -ForegroundColor Green
}
finally {
  Pop-Location
}