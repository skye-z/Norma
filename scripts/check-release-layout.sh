#!/usr/bin/env bash
set -euo pipefail
OUT_DIR="${1:-dist}"
[[ -d "$OUT_DIR" ]] || { echo "[FAIL] output dir missing: $OUT_DIR" >&2; exit 1; }
echo "[OK] output dir exists: $OUT_DIR"
required=("norma-windows-amd64.exe" "norma-linux-amd64" "norma-darwin-amd64")
for f in "${required[@]}"; do [[ -f "$OUT_DIR/$f" ]] && echo "[OK] binary exists: $f" || { echo "[FAIL] binary missing: $f" >&2; exit 1; }; done
for f in release_manifest.json SHA256SUMS.txt INSTALL.md; do [[ -f "$OUT_DIR/$f" ]] && echo "[OK] release artifact exists: $f" || { echo "[FAIL] release artifact missing: $f" >&2; exit 1; }; done
[[ -f config.yaml ]] && echo "[OK] config exists: config.yaml" || echo "[WARN] config missing in workspace: config.yaml"
[[ -d model ]] && echo "[OK] model dir exists: resource/model" || echo "[WARN] model dir missing in workspace: resource/model"
echo "[OK] release layout check passed"