param([string]$Root = ".")
$ErrorActionPreference = "Stop"
$fails = New-Object System.Collections.Generic.List[string]
function Fail([string]$msg) { $fails.Add($msg); Write-Host "[FAIL] $msg" -ForegroundColor Red }
$branch = (git branch --show-current 2>$null)
if ($env:GITHUB_HEAD_REF) { $branch = $env:GITHUB_HEAD_REF }
if ($branch -and $branch -notmatch '^(main|master|codex/.+)$') { Fail "branch naming must be main/master or codex/*, got: $branch" }
if (-not (Test-Path '.github/PULL_REQUEST_TEMPLATE.md')) { Fail 'missing .github/PULL_REQUEST_TEMPLATE.md' }
if (-not (Test-Path 'TODO.md')) { Fail 'missing TODO.md' }
if ($fails.Count -gt 0) { exit 1 }
Write-Host '[OK] change gate passed'