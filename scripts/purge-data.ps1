param(
  [string]$ApiBase = "http://127.0.0.1:8090"
)
$ErrorActionPreference = "Stop"
$resp = Invoke-RestMethod -Method Post -Uri ($ApiBase.TrimEnd('/') + '/privacy/purge') -ContentType 'application/json' -Body '{}'
$resp | ConvertTo-Json -Depth 5
