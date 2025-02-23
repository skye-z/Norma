param([string]$Root = ".")
$ErrorActionPreference = "Stop"
$files = Get-ChildItem -Path $Root -Recurse -Include *.go -File |
  Where-Object { $_.FullName -notmatch '\\(\.gocache|\.gomodcache)\\' }
if (-not $files) {
  Write-Host '[OK] no go files found'
  exit 0
}
$listed = foreach ($file in $files) { & gofmt -l $file.FullName }
if ($LASTEXITCODE -ne 0) {
  exit $LASTEXITCODE
}
$listed = $listed | Where-Object { $_ }
if ($listed) {
  Write-Host '[FAIL] gofmt required for:' -ForegroundColor Red
  $listed
  exit 1
}
Write-Host '[OK] gofmt check passed'