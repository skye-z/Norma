#!/usr/bin/env bash
set -euo pipefail

# Build and verify embedded ASR/Wake (whisper + sherpa) under MSYS2 mingw64.
# Run this script inside "MSYS2 MinGW 64-bit" shell.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

WHISPER_CPP_DIR_DEFAULT="/d/Project/AI/_deps/whisper.cpp"
WHISPER_CPP_DIR="${WHISPER_CPP_DIR:-$WHISPER_CPP_DIR_DEFAULT}"
EMBEDDED_TAGS="${EMBEDDED_TAGS:-whisper_embedded sherpa_embedded}"

log() { printf "[embedded-build] %s\n" "$*"; }
die() { printf "[embedded-build] ERROR: %s\n" "$*" >&2; exit 1; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing command: $1"
}

log "checking required tools"
require_cmd git
require_cmd go
require_cmd gcc
require_cmd cmake
require_cmd make

if [[ ! -d "$WHISPER_CPP_DIR/.git" ]]; then
  log "cloning whisper.cpp -> $WHISPER_CPP_DIR"
  mkdir -p "$(dirname "$WHISPER_CPP_DIR")"
  git clone https://github.com/ggml-org/whisper.cpp.git "$WHISPER_CPP_DIR"
fi

log "building whisper static libs"
pushd "$WHISPER_CPP_DIR/bindings/go" >/dev/null
make whisper
popd >/dev/null

WHISPER_INCLUDE_DIR="$WHISPER_CPP_DIR/include"
GGML_INCLUDE_DIR="$WHISPER_CPP_DIR/ggml/include"
WHISPER_LIB_DIR="$WHISPER_CPP_DIR/build_go/src"
GGML_LIB_DIR="$WHISPER_CPP_DIR/build_go/ggml/src"

[[ -f "$WHISPER_INCLUDE_DIR/whisper.h" ]] || die "missing whisper.h"
[[ -f "$GGML_INCLUDE_DIR/ggml.h" ]] || die "missing ggml.h"
[[ -f "$WHISPER_LIB_DIR/libwhisper.a" ]] || die "missing libwhisper.a"
[[ -f "$GGML_LIB_DIR/ggml.a" ]] || die "missing ggml.a"
[[ -f "$GGML_LIB_DIR/ggml-base.a" ]] || die "missing ggml-base.a"
[[ -f "$GGML_LIB_DIR/ggml-cpu.a" ]] || die "missing ggml-cpu.a"

log "normalizing ggml lib names for cgo linker"
cp -f "$GGML_LIB_DIR/ggml.a" "$GGML_LIB_DIR/libggml.a"
cp -f "$GGML_LIB_DIR/ggml-base.a" "$GGML_LIB_DIR/libggml-base.a"
cp -f "$GGML_LIB_DIR/ggml-cpu.a" "$GGML_LIB_DIR/libggml-cpu.a"

GOMODCACHE="$(go env GOMODCACHE)"
SHERPA_DLL_DIR="$(ls -d "$GOMODCACHE"/github.com/k2-fsa/sherpa-onnx-go-windows@*/lib/x86_64-pc-windows-gnu 2>/dev/null | tail -n 1 || true)"
[[ -n "$SHERPA_DLL_DIR" ]] || die "cannot find sherpa DLL folder in GOMODCACHE"

export CGO_ENABLED=1
export CC=gcc
export CXX=g++
export C_INCLUDE_PATH="$WHISPER_INCLUDE_DIR:$GGML_INCLUDE_DIR${C_INCLUDE_PATH:+:$C_INCLUDE_PATH}"
export LIBRARY_PATH="$WHISPER_LIB_DIR:$GGML_LIB_DIR${LIBRARY_PATH:+:$LIBRARY_PATH}"
export PATH="$SHERPA_DLL_DIR:/mingw64/bin:$PATH"

log "environment"
log "  REPO_ROOT=$REPO_ROOT"
log "  WHISPER_CPP_DIR=$WHISPER_CPP_DIR"
log "  SHERPA_DLL_DIR=$SHERPA_DLL_DIR"
log "  C_INCLUDE_PATH=$C_INCLUDE_PATH"
log "  LIBRARY_PATH=$LIBRARY_PATH"
log "  TAGS=$EMBEDDED_TAGS"

pushd "$REPO_ROOT" >/dev/null
log "running: go test -tags \"$EMBEDDED_TAGS\" ./internal/audio"
go test -tags "$EMBEDDED_TAGS" ./internal/audio
log "running: go test -tags \"$EMBEDDED_TAGS\" ./..."
go test -tags "$EMBEDDED_TAGS" ./...
popd >/dev/null

log "done: embedded build + tests passed"
