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

if (-not (Test-Path $OutDir)) {
  New-Item -ItemType Directory -Path $OutDir | Out-Null
}

$commit = ""
try { $commit = (git rev-parse --short HEAD) } catch { $commit = "unknown" }
$ldflags = "-X main.version=$Version -X main.commit=$commit"

foreach ($t in $targets) {
  $name = "norma-$($t.GOOS)-$($t.GOARCH)$($t.EXE)"
  $out = Join-Path $OutDir $name

  Write-Host "Building $name ..."
  $env:GOOS = $t.GOOS
  $env:GOARCH = $t.GOARCH
  go build -trimpath -ldflags $ldflags -o $out ./cmd/norma
}

Write-Host "Build complete. Output => $OutDir"
