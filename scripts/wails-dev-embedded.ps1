param(
  [string]$MSYS2Shell = "D:\Environment\MSYS2\msys2_shell.cmd",
  [string]$WhisperCppDir = "D:/Project/AI/_deps/whisper.cpp",
  [string]$Tags = "whisper_embedded,sherpa_embedded"
)

$ErrorActionPreference = "Stop"
if (-not (Test-Path $MSYS2Shell)) {
  throw "msys2_shell.cmd not found: $MSYS2Shell"
}

function Convert-ToMsysPath {
  param([Parameter(Mandatory = $true)][string]$Path)

  $normalized = $Path.Trim() -replace '\\', '/'
  if ($normalized -match '^([A-Za-z]):(.*)$') {
    return ('/{0}{1}' -f $matches[1].ToLowerInvariant(), $matches[2])
  }
  return $normalized
}

function Find-SherpaDllDir {
  $candidates = @()
  if ($env:GOMODCACHE) { $candidates += $env:GOMODCACHE }
  $localMod = Join-Path (Get-Location) '.gomodcache'
  if (Test-Path $localMod) { $candidates += $localMod }
  $goEnvMod = (& go env GOMODCACHE 2>$null)
  if ($goEnvMod) { $candidates += $goEnvMod.Trim() }
  if ($env:GOPATH) {
    $candidates += (Join-Path $env:GOPATH 'pkg\mod')
    $candidates += (Join-Path $env:GOPATH 'Library')
  }

  foreach ($root in ($candidates | Where-Object { $_ } | Select-Object -Unique)) {
    $pattern = Join-Path $root 'github.com\k2-fsa\sherpa-onnx-go-windows@*\lib\x86_64-pc-windows-gnu'
    $match = Get-ChildItem -Path $pattern -Directory -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($match -and (Test-Path (Join-Path $match.FullName 'onnxruntime.dll'))) {
      return $match.FullName
    }
  }
  return $null
}

$scriptPath = Convert-ToMsysPath (Join-Path $PSScriptRoot 'embedded-env-mingw64.sh')
$repoRoot = Convert-ToMsysPath (Split-Path -Parent $PSScriptRoot)
$hostGoBinWin = Split-Path (Get-Command go).Source -Parent
$hostWailsBinWin = Split-Path (Get-Command wails).Source -Parent
$hostNodeBinWin = Split-Path (Get-Command npm).Source -Parent
$sherpaDllDirWin = Find-SherpaDllDir
if ($sherpaDllDirWin) {
  $env:SHERPA_DLL_DIR_WIN = $sherpaDllDirWin
  $env:PATH = "$sherpaDllDirWin;$env:PATH"
  Write-Host "[embedded] SHERPA_DLL_DIR(win)=$sherpaDllDirWin"
}
$hostGoBin = Convert-ToMsysPath $hostGoBinWin
$hostWailsBin = Convert-ToMsysPath $hostWailsBinWin
$hostNodeBin = Convert-ToMsysPath $hostNodeBinWin
$embeddedTags = $Tags -replace ',', ' '
$cmd = "cd '$repoRoot' && export HOST_GO_BIN='$hostGoBin' && export HOST_WAILS_BIN='$hostWailsBin' && export HOST_NODE_BIN='$hostNodeBin' && export WHISPER_CPP_DIR='$WhisperCppDir' && export EMBEDDED_TAGS='$embeddedTags' && '$scriptPath' wails dev -tags '$embeddedTags'"

Write-Host "[embedded] wails dev with tags: $Tags"
& $MSYS2Shell -defterm -no-start -mingw64 -here -c $cmd
exit $LASTEXITCODE
