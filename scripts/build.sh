#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-dev}"
OUT_DIR="${2:-dist}"

mkdir -p "$OUT_DIR"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
LDFLAGS="-X main.version=$VERSION -X main.commit=$COMMIT"

build() {
  local goos="$1" goarch="$2" exe="$3"
  local name="norma-${goos}-${goarch}${exe}"
  echo "Building ${name} ..."
  GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "$LDFLAGS" -o "${OUT_DIR}/${name}" ./cmd/norma
}

build windows amd64 .exe
build linux amd64 ""
build darwin amd64 ""

echo "Build complete. Output => ${OUT_DIR}"
