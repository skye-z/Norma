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
if [[ ! -d "frontend/dist/console" ]]; then
  (cd frontend && npm run build)
fi
for target in windows/amd64 linux/amd64 darwin/amd64; do
  GOOS="${target%/*}"
  GOARCH="${target#*/}"
  PKG_DIR="$OUT_DIR/Norma-${GOOS}-${GOARCH}"
  rm -rf "$PKG_DIR"
  mkdir -p "$PKG_DIR/resource/runtime/providers" "$PKG_DIR/resource/data" "$PKG_DIR/resource/logs" "$PKG_DIR/resource/diagnostics"
  if [[ "$GOOS" == "windows" ]]; then
    BINARY_NAME="Norma.exe"
  else
    BINARY_NAME="Norma"
  fi
  echo "Building ${BINARY_NAME} for ${GOOS}/${GOARCH} ..."
  GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -ldflags "$LDFLAGS" -o "$PKG_DIR/$BINARY_NAME" .
  cp config.example.yaml "$PKG_DIR/config.example.yaml"
  cp config.example.yaml "$PKG_DIR/config.yaml"
  cp -R frontend/dist/console "$PKG_DIR/console"
  if [[ -d "resource/model" ]]; then
    cp -R resource/model "$PKG_DIR/resource/model"
  fi
  if [[ "$GOOS" == "windows" ]]; then
    if [[ ! -d "build/embedded/tts/kokoro/windows" ]]; then
      echo "embedded kokoro assets missing: build/embedded/tts/kokoro/windows" >&2
      exit 1
    fi
    mkdir -p "$PKG_DIR/resource/runtime/providers/tts/current"
    cp -R build/embedded/tts/kokoro/windows/* "$PKG_DIR/resource/runtime/providers/tts/current/"
  fi
done
bash ./scripts/write-release-manifest.sh "$OUT_DIR" "$VERSION"
bash ./scripts/check-release-layout.sh "$OUT_DIR"
