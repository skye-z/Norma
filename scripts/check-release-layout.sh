#!/usr/bin/env bash
set -euo pipefail
OUT_DIR="${1:-dist}"
[[ -d "$OUT_DIR" ]] || { echo "[FAIL] output dir missing: $OUT_DIR" >&2; exit 1; }
echo "[OK] output dir exists: $OUT_DIR"
for pkg in Norma-windows-amd64 Norma-linux-amd64 Norma-darwin-amd64; do
  [[ -d "$OUT_DIR/$pkg" ]] || { echo "[FAIL] package missing: $pkg" >&2; exit 1; }
  echo "[OK] package exists: $pkg"
  if [[ "$pkg" == Norma-windows-* ]]; then
    binary="Norma.exe"
  else
    binary="Norma"
  fi
  for path in "$binary" config.example.yaml config.yaml debug resource/data resource/logs resource/diagnostics resource/runtime/providers; do
    [[ -e "$OUT_DIR/$pkg/$path" ]] && echo "[OK] $pkg => $path" || { echo "[FAIL] $pkg missing: $path" >&2; exit 1; }
  done
  if [[ -d "resource/model" ]]; then
    [[ -d "$OUT_DIR/$pkg/resource/model" ]] && echo "[OK] $pkg => resource/model" || { echo "[FAIL] $pkg missing: resource/model" >&2; exit 1; }
  fi
done
for f in release_manifest.json SHA256SUMS.txt INSTALL.md; do [[ -f "$OUT_DIR/$f" ]] && echo "[OK] release artifact exists: $f" || { echo "[FAIL] release artifact missing: $f" >&2; exit 1; }; done
echo "[OK] release layout check passed"
