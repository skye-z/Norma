#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="${1:-dist}"
VERSION="${2:-dev}"
TMP_FILE="${OUT_DIR}/SHA256SUMS.txt"
: > "$TMP_FILE"
PACKAGE_JSON=""
for pkg in "$OUT_DIR"/*; do
  [[ -d "$pkg" ]] || continue
  pkg_name="$(basename "$pkg")"
  FILE_JSON=""
  while IFS= read -r -d '' file; do
    rel="${file#$pkg/}"
    sha="$(sha256sum "$file" | awk '{print $1}')"
    size="$(wc -c < "$file")"
    printf '%s  %s/%s\n' "$sha" "$pkg_name" "$rel" >> "$TMP_FILE"
    entry="{\"path\":\"$rel\",\"size\":$size,\"sha256\":\"$sha\"}"
    if [[ -n "$FILE_JSON" ]]; then
      FILE_JSON+=" , $entry"
    else
      FILE_JSON="$entry"
    fi
    if [[ -n "${NORMA_SIGN_FILE_CMD:-}" ]]; then
      eval "${NORMA_SIGN_FILE_CMD//\{file\}/\"$file\"}"
      printf '%s/%s\n' "$pkg_name" "$rel" >> "${OUT_DIR}/SIGNATURES.txt"
    fi
  done < <(find "$pkg" -type f -print0)
  package_entry="{\"name\":\"$pkg_name\",\"files\":[ $FILE_JSON ]}"
  if [[ -n "$PACKAGE_JSON" ]]; then
    PACKAGE_JSON+=" , $package_entry"
  else
    PACKAGE_JSON="$package_entry"
  fi
done
cat > "${OUT_DIR}/release_manifest.json" <<JSON
{
  "version": "${VERSION}",
  "packages": [ ${PACKAGE_JSON} ]
}
JSON
cat > "${OUT_DIR}/INSTALL.md" <<'MD'
# Norma Install Guide

Each package directory uses this layout:
- `Norma.exe` or `Norma`
- `config.example.yaml`
- `config.yaml`
- `console/`
- `resource/model/`
- `resource/data/`
- `resource/logs/`
- `resource/diagnostics/`
- `resource/runtime/providers/`
- `resource/runtime/providers/tts/current/model.onnx` (Windows embedded)

On first run, edit `config.yaml` if needed, then start Norma.
MD

echo "[OK] release manifest written"
