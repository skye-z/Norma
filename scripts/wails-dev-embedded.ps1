param(
  [string]$MSYS2Shell = "D:\Environment\MSYS2\msys2_shell.cmd",
  [string]$WhisperCppDir = "D:/Project/AI/_deps/whisper.cpp",
  [string]$Tags = "whisper_embedded,sherpa_embedded"
)

$ErrorActionPreference = "Stop"
if (-not (Test-Path $MSYS2Shell)) {
  throw "msys2_shell.cmd not found: $MSYS2Shell"
}

$scriptPath = (Join-Path $PSScriptRoot 'embedded-env-mingw64.sh').Replace("\", "/")
$repoRoot = (Split-Path -Parent $PSScriptRoot).Replace("\", "/")
$hostGoBinWin = (Split-Path (Get-Command go).Source -Parent)
$hostWailsBinWin = (Split-Path (Get-Command wails).Source -Parent)
$hostNodeBinWin = (Split-Path (Get-Command npm).Source -Parent)
$hostGoBin = (& $MSYS2Shell -defterm -no-start -mingw64 -here -c "cygpath -u '$($hostGoBinWin -replace '\\','/')'").Trim()
$hostWailsBin = (& $MSYS2Shell -defterm -no-start -mingw64 -here -c "cygpath -u '$($hostWailsBinWin -replace '\\','/')'").Trim()
$hostNodeBin = (& $MSYS2Shell -defterm -no-start -mingw64 -here -c "cygpath -u '$($hostNodeBinWin -replace '\\','/')'").Trim()
$cmd = "cd '$repoRoot' && export HOST_GO_BIN='$hostGoBin' && export HOST_WAILS_BIN='$hostWailsBin' && export HOST_NODE_BIN='$hostNodeBin' && export WHISPER_CPP_DIR='$WhisperCppDir' && export EMBEDDED_TAGS='whisper_embedded sherpa_embedded' && '$scriptPath' wails dev -tags '$($Tags -replace ',', ' ')'"

Write-Host "[embedded] wails dev with tags: $Tags"
& $MSYS2Shell -defterm -no-start -mingw64 -here -c $cmd
exit $LASTEXITCODE
