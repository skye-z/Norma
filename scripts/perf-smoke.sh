#!/usr/bin/env bash
set -euo pipefail
ROOT="${1:-.}"
BASELINE="${2:-configs/perf-baseline.json}"
[[ -f "$BASELINE" ]] || { echo "baseline missing: $BASELINE" >&2; exit 1; }
LOG_PATH="$ROOT/logs/app.log"
[[ -f "$LOG_PATH" ]] || { echo "log missing: $LOG_PATH" >&2; exit 1; }
echo "[OK] perf smoke placeholder passed"