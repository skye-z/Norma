#!/usr/bin/env bash
set -euo pipefail
VERSION="${1:-dev}"
OUT_DIR="${2:-dist}"
mkdir -p "$OUT_DIR"
export GOCACHE="${PWD}/.gocache"
export GOMODCACHE="${PWD}/.gomodcache"
mkdir -p "$GOCACHE" "$GOMODCACHE"
COMMIT="unknown"
if git rev-parse --short HEAD >/dev/null 2>&1; then
  COMMIT="$(git rev-parse --short HEAD)"
fi
LDFLAGS="-X main.version=${VERSION} -X main.commit=${COMMIT}"
for target in windows/amd64 linux/amd64 darwin/amd64; do
  GOOS="${target%/*}"
  GOARCH="${target#*/}"
  EXT=""
  if [[ "$GOOS" == "windows" ]]; then EXT=".exe"; fi
  NAME="norma-${GOOS}-${GOARCH}${EXT}"
  echo "Building ${NAME} ..."
  GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -ldflags "$LDFLAGS" -o "$OUT_DIR/$NAME" ./cmd/norma
done
bash ./scripts/write-release-manifest.sh "$OUT_DIR" "$VERSION"
bash ./scripts/check-release-layout.sh "$OUT_DIR"