#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

WHISPER_CPP_DIR_DEFAULT="/d/Project/AI/Norma/deps/whisper.cpp"
WHISPER_CPP_DIR="${WHISPER_CPP_DIR:-$WHISPER_CPP_DIR_DEFAULT}"
EMBEDDED_TAGS="${EMBEDDED_TAGS:-whisper_embedded sherpa_embedded}"
HOST_GO_BIN="${HOST_GO_BIN:-}"
HOST_WAILS_BIN="${HOST_WAILS_BIN:-}"
HOST_NODE_BIN="${HOST_NODE_BIN:-}"

log() { printf "[embedded-env] %s\n" "$*"; }
die() { printf "[embedded-env] ERROR: %s\n" "$*" >&2; exit 1; }
require_cmd() { command -v "$1" >/dev/null 2>&1 || die "missing command: $1"; }

find_vulkan_sdk() {
  if [[ -n "${VULKAN_SDK:-}" && -x "$VULKAN_SDK/Bin/glslc.exe" ]]; then
    printf "%s" "$VULKAN_SDK"
    return 0
  fi

  local candidates=(/c/VulkanSDK/*)
  local sdk
  for sdk in "${candidates[@]}"; do
    [[ -d "$sdk" ]] || continue
    if [[ -x "$sdk/Bin/glslc.exe" ]]; then
      printf "%s" "$sdk"
      return 0
    fi
  done

  return 1
}

detect_whisper_gpu_backend() {
  if ! command -v vulkaninfo >/dev/null 2>&1; then
    log "whisper GPU disabled: vulkaninfo not found"
    return 1
  fi

  local sdk
  sdk="$(find_vulkan_sdk || true)"
  if [[ -z "$sdk" ]]; then
    log "whisper GPU disabled: Vulkan SDK with glslc was not found"
    return 1
  fi

  export VULKAN_SDK="$sdk"
  export PATH="$sdk/Bin:$PATH"
  if ! command -v glslc >/dev/null 2>&1; then
    log "whisper GPU disabled: glslc not found in VULKAN_SDK=$sdk"
    return 1
  fi

  export NORMA_WHISPER_GPU_BACKEND="vulkan"
  log "whisper GPU backend enabled: vulkan (VULKAN_SDK=$sdk)"
  return 0
}

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

  local whisper_gpu_backend="cpu"
  if detect_whisper_gpu_backend; then
    whisper_gpu_backend="$NORMA_WHISPER_GPU_BACKEND"
  fi

  log "building whisper static libs (backend=$whisper_gpu_backend)"
  pushd "$WHISPER_CPP_DIR/bindings/go" >/dev/null
  if [[ "$whisper_gpu_backend" == "vulkan" ]]; then
    cmake -S ../.. -B ../../${BUILD_DIR} \
      -DCMAKE_BUILD_TYPE=Release \
      -DBUILD_SHARED_LIBS=OFF \
      -DGGML_VULKAN=ON
    cmake --build ../../${BUILD_DIR} --target whisper
  else
    make whisper
  fi
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

  if [[ "${NORMA_WHISPER_GPU_BACKEND:-cpu}" == "vulkan" ]]; then
    [[ -f "$ggml_lib_dir/ggml-vulkan.a" ]] || die "missing ggml-vulkan.a"
    cp -f "$ggml_lib_dir/ggml-vulkan.a" "$ggml_lib_dir/libggml-vulkan.a"
  fi

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
  if [[ "${NORMA_WHISPER_GPU_BACKEND:-cpu}" == "vulkan" ]]; then
    export LIBRARY_PATH="$whisper_lib_dir:$ggml_lib_dir:${VULKAN_SDK}/Lib${LIBRARY_PATH:+:$LIBRARY_PATH}"
    export CGO_LDFLAGS="-L$whisper_lib_dir -L$ggml_lib_dir -L${VULKAN_SDK}/Lib -lggml-vulkan -lvulkan-1${CGO_LDFLAGS:+ $CGO_LDFLAGS}"
  fi
  export PATH="$sherpa_dll_dir:/mingw64/bin:$PATH"

  log "environment ready"
  log "  WHISPER_CPP_DIR=$WHISPER_CPP_DIR"
  log "  SHERPA_DLL_DIR=$sherpa_dll_dir"
  log "  TAGS=$EMBEDDED_TAGS"
  log "  WHISPER_GPU_BACKEND=${NORMA_WHISPER_GPU_BACKEND:-cpu}"
  if [[ -n "${VULKAN_SDK:-}" ]]; then
    log "  VULKAN_SDK=$VULKAN_SDK"
  fi
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
