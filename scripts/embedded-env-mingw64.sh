#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

WHISPER_CPP_DIR_DEFAULT="/d/Project/AI/_deps/whisper.cpp"
WHISPER_CPP_DIR="${WHISPER_CPP_DIR:-$WHISPER_CPP_DIR_DEFAULT}"
EMBEDDED_TAGS="${EMBEDDED_TAGS:-whisper_embedded sherpa_embedded}"
HOST_GO_BIN="${HOST_GO_BIN:-}"
HOST_WAILS_BIN="${HOST_WAILS_BIN:-}"
HOST_NODE_BIN="${HOST_NODE_BIN:-}"

log() { printf "[embedded-env] %s\n" "$*"; }
die() { printf "[embedded-env] ERROR: %s\n" "$*" >&2; exit 1; }
require_cmd() { command -v "$1" >/dev/null 2>&1 || die "missing command: $1"; }

prepare_embedded_env() {
  if [[ -n "$HOST_GO_BIN" ]]; then
    export PATH="$HOST_GO_BIN:$PATH"
  fi
  if [[ -n "$HOST_WAILS_BIN" ]]; then
    export PATH="$HOST_WAILS_BIN:$PATH"
  fi
  if [[ -n "$HOST_NODE_BIN" ]]; then
    export PATH="$HOST_NODE_BIN:$PATH"
  fi
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

  local whisper_include_dir="$WHISPER_CPP_DIR/include"
  local ggml_include_dir="$WHISPER_CPP_DIR/ggml/include"
  local whisper_lib_dir="$WHISPER_CPP_DIR/build_go/src"
  local ggml_lib_dir="$WHISPER_CPP_DIR/build_go/ggml/src"

  [[ -f "$whisper_include_dir/whisper.h" ]] || die "missing whisper.h"
  [[ -f "$ggml_include_dir/ggml.h" ]] || die "missing ggml.h"
  [[ -f "$whisper_lib_dir/libwhisper.a" ]] || die "missing libwhisper.a"
  [[ -f "$ggml_lib_dir/ggml.a" ]] || die "missing ggml.a"
  [[ -f "$ggml_lib_dir/ggml-base.a" ]] || die "missing ggml-base.a"
  [[ -f "$ggml_lib_dir/ggml-cpu.a" ]] || die "missing ggml-cpu.a"

  cp -f "$ggml_lib_dir/ggml.a" "$ggml_lib_dir/libggml.a"
  cp -f "$ggml_lib_dir/ggml-base.a" "$ggml_lib_dir/libggml-base.a"
  cp -f "$ggml_lib_dir/ggml-cpu.a" "$ggml_lib_dir/libggml-cpu.a"

  local gomodcache
  gomodcache="$(go env GOMODCACHE)"
  local sherpa_dll_dir
  sherpa_dll_dir="$(ls -d "$gomodcache"/github.com/k2-fsa/sherpa-onnx-go-windows@*/lib/x86_64-pc-windows-gnu 2>/dev/null | tail -n 1 || true)"
  [[ -n "$sherpa_dll_dir" ]] || die "cannot find sherpa DLL folder in GOMODCACHE"

  export CGO_ENABLED=1
  export CC=gcc
  export CXX=g++
  export C_INCLUDE_PATH="$whisper_include_dir:$ggml_include_dir${C_INCLUDE_PATH:+:$C_INCLUDE_PATH}"
  export LIBRARY_PATH="$whisper_lib_dir:$ggml_lib_dir${LIBRARY_PATH:+:$LIBRARY_PATH}"
  export CGO_CFLAGS="-I$whisper_include_dir -I$ggml_include_dir${CGO_CFLAGS:+ $CGO_CFLAGS}"
  export CGO_CXXFLAGS="-I$whisper_include_dir -I$ggml_include_dir${CGO_CXXFLAGS:+ $CGO_CXXFLAGS}"
  export CGO_LDFLAGS="-L$whisper_lib_dir -L$ggml_lib_dir${CGO_LDFLAGS:+ $CGO_LDFLAGS}"
  export PATH="$sherpa_dll_dir:/mingw64/bin:$PATH"

  log "environment ready"
  log "  WHISPER_CPP_DIR=$WHISPER_CPP_DIR"
  log "  SHERPA_DLL_DIR=$sherpa_dll_dir"
  log "  TAGS=$EMBEDDED_TAGS"
  log "  CGO_CFLAGS=$CGO_CFLAGS"
  log "  CGO_LDFLAGS=$CGO_LDFLAGS"
}

prepare_embedded_env

if [[ $# -eq 0 ]]; then
  die "no command provided"
fi

pushd "$REPO_ROOT" >/dev/null
log "running: $*"
"$@"
popd >/dev/null
