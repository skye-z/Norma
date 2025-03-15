param(
  [string]$Version = "dev",
  [string]$OutDir = "dist"
)

$ErrorActionPreference = "Stop"
$targets = @(
  @{ GOOS = "windows"; GOARCH = "amd64"; EXE = ".exe" },
  @{ GOOS = "linux"; GOARCH = "amd64"; EXE = "" },
  @{ GOOS = "darwin"; GOARCH = "amd64"; EXE = "" }
)
if (-not (Test-Path $OutDir)) { New-Item -ItemType Directory -Path $OutDir | Out-Null }
$env:GOCACHE = Join-Path (Get-Location) '.gocache'
$env:GOMODCACHE = Join-Path (Get-Location) '.gomodcache'
if (-not (Test-Path $env:GOCACHE)) { New-Item -ItemType Directory -Path $env:GOCACHE | Out-Null }
if (-not (Test-Path $env:GOMODCACHE)) { New-Item -ItemType Directory -Path $env:GOMODCACHE | Out-Null }
$commit = ""
try { $commit = (git rev-parse --short HEAD) } catch { $commit = "unknown" }
$ldflags = "-X main.version=$Version -X main.commit=$commit"
foreach ($t in $targets) {
  $packageDir = Join-Path $OutDir "Norma-$($t.GOOS)-$($t.GOARCH)"
  if (Test-Path $packageDir) { Remove-Item -Recurse -Force $packageDir }
  New-Item -ItemType Directory -Path $packageDir | Out-Null
  $binaryName = if ($t.GOOS -eq 'windows') { "Norma.exe" } else { "Norma" }
  $out = Join-Path $packageDir $binaryName
  Write-Host "Building $($t.GOOS)-$($t.GOARCH) ..."
  $env:GOOS = $t.GOOS
  $env:GOARCH = $t.GOARCH
  & go build -trimpath -ldflags $ldflags -o $out ./cmd/norma
  if ($LASTEXITCODE -ne 0) { throw "go build failed for $($t.GOOS)-$($t.GOARCH)" }

  Copy-Item -Force config.example.yaml (Join-Path $packageDir 'config.example.yaml')
  Copy-Item -Force config.example.yaml (Join-Path $packageDir 'config.yaml')

  $resourceRoot = Join-Path $packageDir 'resource'
  New-Item -ItemType Directory -Force -Path $resourceRoot | Out-Null
  foreach ($dir in @('data','logs','diagnostics','runtime/providers')) {
    New-Item -ItemType Directory -Force -Path (Join-Path $resourceRoot $dir) | Out-Null
  }
  if (Test-Path 'resource/model') {
    Copy-Item -Recurse -Force 'resource/model' (Join-Path $resourceRoot 'model')
  }
}
Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
Write-Host "Build complete. Output => $OutDir"
powershell -ExecutionPolicy Bypass -File scripts/write-release-manifest.ps1 -OutDir $OutDir -Version $Version
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Running release layout check ..."
powershell -ExecutionPolicy Bypass -File scripts/check-release-layout.ps1 -OutDir $OutDir
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }