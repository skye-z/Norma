param(
  [string]$OutDir = "diagnostics/audio-device-matrix"
)

$ErrorActionPreference = "Stop"
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$outFile = Join-Path $OutDir ("audio-devices-" + (Get-Date -Format 'yyyyMMdd-HHmmss') + ".json")
$items = @()
try {
  $items = Get-CimInstance Win32_SoundDevice | ForEach-Object {
    [ordered]@{
      name = $_.Name
      manufacturer = $_.Manufacturer
      status = $_.Status
      pnp_device_id = $_.PNPDeviceID
    }
  }
} catch {
  $items = @([ordered]@{ error = $_.Exception.Message })
}
$report = [ordered]@{
  generated_at = (Get-Date).ToString('s')
  host = $env:COMPUTERNAME
  devices = $items
  fallback_modes = @(
    'wake_mode=vad_only',
    'audio.scene=headset',
    'audio.aec_enabled=false + audio.ns_enabled=false',
    'tts.voice=zf_xiaobei'
  )
}
$report | ConvertTo-Json -Depth 5 | Set-Content -Path $outFile -Encoding utf8
Write-Host $outFile
