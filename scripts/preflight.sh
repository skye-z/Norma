#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-.}"
CONFIG="${2:-config.yaml}"
MODEL_DIR="${3:-model}"

if command -v pwsh >/dev/null 2>&1; then
  exec pwsh "$ROOT/scripts/preflight.ps1" -Root "$ROOT" -ConfigPath "$CONFIG" -ModelDir "$MODEL_DIR"
fi

if command -v powershell >/dev/null 2>&1; then
  exec powershell -ExecutionPolicy Bypass -File "$ROOT/scripts/preflight.ps1" -Root "$ROOT" -ConfigPath "$CONFIG" -ModelDir "$MODEL_DIR"
fi

echo "[FAIL] Neither pwsh nor powershell is available." >&2
exit 1