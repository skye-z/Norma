#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-.}"

if command -v pwsh >/dev/null 2>&1; then
  exec pwsh "$ROOT/scripts/diagnose.ps1" -Root "$ROOT"
fi

if command -v powershell >/dev/null 2>&1; then
  exec powershell -ExecutionPolicy Bypass -File "$ROOT/scripts/diagnose.ps1" -Root "$ROOT"
fi

echo "[FAIL] Neither pwsh nor powershell is available." >&2
exit 1