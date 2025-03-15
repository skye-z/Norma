#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-.}"
CONFIG="${2:-config.yaml}"

if command -v pwsh >/dev/null 2>&1; then
  exec pwsh "$ROOT/scripts/watchdog.ps1" -Root "$ROOT" -ConfigPath "$CONFIG"
fi

if command -v powershell >/dev/null 2>&1; then
  exec powershell -ExecutionPolicy Bypass -File "$ROOT/scripts/watchdog.ps1" -Root "$ROOT" -ConfigPath "$CONFIG"
fi

echo "[FAIL] Neither pwsh nor powershell is available." >&2
exit 1