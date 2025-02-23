#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="${1:-dist}"
VERSION="${2:-dev}"
TMP_FILE="${OUT_DIR}/SHA256SUMS.txt"
: > "$TMP_FILE"
ARTIFACT_JSON=""
for file in "$OUT_DIR"/*; do
  [[ -f "$file" ]] || continue
  name="$(basename "$file")"
  sha="$(sha256sum "$file" | awk '{print $1}')"
  size="$(wc -c < "$file")"
  printf '%s  %s\n' "$sha" "$name" >> "$TMP_FILE"
  entry="{\"name\":\"$name\",\"size\":$size,\"sha256\":\"$sha\"}"
  if [[ -n "$ARTIFACT_JSON" ]]; then
    ARTIFACT_JSON+=" , $entry"
  else
    ARTIFACT_JSON="$entry"
  fi
  if [[ -n "${NORMA_SIGN_FILE_CMD:-}" ]]; then
    eval "${NORMA_SIGN_FILE_CMD//\{file\}/\"$file\"}"
    printf '%s\n' "$name" >> "${OUT_DIR}/SIGNATURES.txt"
  fi
done
cat > "${OUT_DIR}/release_manifest.json" <<JSON
{
  "version": "${VERSION}",
  "artifacts": [ ${ARTIFACT_JSON} ]
}
JSON
cat > "${OUT_DIR}/INSTALL.md" <<'MD'
# Norma Install Guide

## Artifacts

Place the selected binary together with:
- `configs/config.yaml`
- `model/`
- local runtime dependencies if using Whisper.cpp / XTTS / Ollama

## First Run

```bash
bash ./scripts/start.sh . configs/config.yaml
```
MD

echo "[OK] release manifest written"