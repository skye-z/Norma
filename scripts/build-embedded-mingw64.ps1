param(
    [string]$MSYS2Shell = "D:\Environment\MSYS2\msys2_shell.cmd",
    [string]$WhisperCppDir = "D:/Project/AI/Norma/deps/whisper.cpp"
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $MSYS2Shell)) {
    throw "msys2_shell.cmd not found: $MSYS2Shell"
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$scriptPath = ($PSScriptRoot + "/build-embedded-mingw64.sh").Replace("\", "/")
$repoMsys = $repoRoot.Replace("\", "/")

$cmd = "export WHISPER_CPP_DIR='$WhisperCppDir'; '$scriptPath'"

& $MSYS2Shell -defterm -no-start -mingw64 -here -c $cmd
if ($LASTEXITCODE -ne 0) {
    throw "embedded mingw64 build failed with code $LASTEXITCODE"
}

Write-Host "[embedded-build] done" -ForegroundColor Green
