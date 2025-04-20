param(
  [Parameter(Position = 0)]
  [string]$Binary
)

$ErrorActionPreference = 'Stop'

function Find-SherpaDllDir {
  $candidates = @()
  if ($env:SHERPA_DLL_DIR_WIN) { $candidates += $env:SHERPA_DLL_DIR_WIN }
  if ($env:SHERPA_DLL_DIR) { $candidates += $env:SHERPA_DLL_DIR }
  if ($env:GOMODCACHE) { $candidates += $env:GOMODCACHE }

  $repoRoot = Resolve-Path (Join-Path $PSScriptRoot '..')
  $localMod = Join-Path $repoRoot '.gomodcache'
  if (Test-Path $localMod) { $candidates += $localMod }

  $goEnvMod = (& go env GOMODCACHE 2>$null)
  if ($goEnvMod) { $candidates += $goEnvMod.Trim() }

  if ($env:GOPATH) {
    $candidates += (Join-Path $env:GOPATH 'pkg\mod')
    $candidates += (Join-Path $env:GOPATH 'Library')
  }

  foreach ($root in ($candidates | Where-Object { $_ } | Select-Object -Unique)) {
    if (Test-Path (Join-Path $root 'onnxruntime.dll')) {
      return (Resolve-Path $root).Path
    }

    $pattern = Join-Path $root 'github.com\k2-fsa\sherpa-onnx-go-windows@*\lib\x86_64-pc-windows-gnu'
    $match = Get-ChildItem -Path $pattern -Directory -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($match -and (Test-Path (Join-Path $match.FullName 'onnxruntime.dll'))) {
      return $match.FullName
    }
  }

  throw 'cannot find sherpa runtime DLL directory'
}

$dllDir = Find-SherpaDllDir
$targetDir = if ($Binary) {
  Split-Path -Parent $Binary
} else {
  Resolve-Path (Join-Path $PSScriptRoot '..\build\bin')
}

if (-not (Test-Path $targetDir)) {
  New-Item -ItemType Directory -Path $targetDir -Force | Out-Null
}

$copied = @()
foreach ($name in @('onnxruntime.dll', 'sherpa-onnx-c-api.dll', 'sherpa-onnx-cxx-api.dll')) {
  $src = Join-Path $dllDir $name
  if (-not (Test-Path $src)) {
    throw "missing sherpa runtime file: $src"
  }
  Copy-Item -Path $src -Destination (Join-Path $targetDir $name) -Force
  $copied += $name
}

Write-Host "[embedded] copied sherpa runtime to $targetDir => $($copied -join ', ')"
