#!/usr/bin/env bash
set -euo pipefail
ROOT="${1:-.}"
CONFIG="${2:-config.yaml}"
cd "$ROOT"
export GOCACHE="${PWD}/.gocache"
export GOMODCACHE="${PWD}/.gomodcache"
mkdir -p "$GOCACHE" "$GOMODCACHE" diagnostics/beta-smoke

echo "[beta] run focused tests"
go test ./internal/index ./internal/memory ./internal/runtime ./internal/api

LOG_FILE="diagnostics/beta-smoke/beta-$(date +%Y%m%d-%H%M%S).log"
echo "[beta] run norma demo"
NORMA_LLM_PROVIDER=mock go run ./cmd/norma -config "$CONFIG" -demo >"$LOG_FILE" 2>&1

for marker in 'rag.ready' 'llm.token' 'tts.chunk' 'metric] summary'; do
  if ! grep -q "$marker" "$LOG_FILE"; then
    echo "[beta] missing demo marker: $marker" >&2
    exit 1
  fi
done

echo "[beta] ok => $LOG_FILE"