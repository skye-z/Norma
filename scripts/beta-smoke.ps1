param(
  [string]$Root = ".",
  [string]$Config = "config.yaml"
)

$ErrorActionPreference = "Stop"
Push-Location $Root
try {
  $env:GOCACHE = Join-Path (Get-Location) '.gocache'
  $env:GOMODCACHE = Join-Path (Get-Location) '.gomodcache'
  New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
  New-Item -ItemType Directory -Force -Path $env:GOMODCACHE | Out-Null

  Write-Host "[beta] run focused tests"
  go test ./internal/index ./internal/memory ./internal/runtime ./internal/api
  if ($LASTEXITCODE -ne 0) { throw "focused go test failed" }

  $outDir = Join-Path 'diagnostics' 'beta-smoke'
  New-Item -ItemType Directory -Force -Path $outDir | Out-Null
  $logFile = Join-Path $outDir ("beta-" + (Get-Date -Format 'yyyyMMdd-HHmmss') + ".log")

  Write-Host "[beta] run norma demo"
  $env:NORMA_LLM_PROVIDER = 'mock'
  go run ./cmd/norma -config $Config -demo *> $logFile
  Remove-Item Env:NORMA_LLM_PROVIDER -ErrorAction SilentlyContinue
  if ($LASTEXITCODE -ne 0) { throw "go run demo failed" }

  $required = @('rag.ready', 'llm.token', 'tts.chunk', 'metric] summary')
  foreach ($pattern in $required) {
    if (-not (Select-String -Path $logFile -Pattern $pattern -Quiet)) {
      throw "missing demo marker: $pattern"
    }
  }

  Write-Host "[beta] ok => $logFile"
} finally {
  Pop-Location
}