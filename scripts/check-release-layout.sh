#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="${1:-dist}"

if [[ ! -d "$OUT_DIR" ]]; then
  echo "[FAIL] output dir missing: $OUT_DIR" >&2
  exit 1
fi

echo "[OK] output dir exists: $OUT_DIR"

required=(
  "norma-windows-amd64.exe"
  "norma-linux-amd64"
  "norma-darwin-amd64"
)

for f in "${required[@]}"; do
  if [[ -f "$OUT_DIR/$f" ]]; then
    echo "[OK] binary exists: $f"
  else
    echo "[FAIL] binary missing: $f" >&2
    exit 1
  fi
done

if [[ -f "configs/config.yaml" ]]; then
  echo "[OK] config exists: configs/config.yaml"
else
  echo "[WARN] config missing in workspace: configs/config.yaml"
fi

if [[ -d "model" ]]; then
  echo "[OK] model dir exists: model"
else
  echo "[WARN] model dir missing in workspace: model"
fi

echo "[OK] release layout check passed"