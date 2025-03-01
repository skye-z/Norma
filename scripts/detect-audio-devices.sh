#!/usr/bin/env bash
set -euo pipefail
OUT_DIR="${1:-diagnostics/audio-device-matrix}"
mkdir -p "$OUT_DIR"
OUT_FILE="$OUT_DIR/audio-devices-$(date +%Y%m%d-%H%M%S).txt"
{
  echo "generated_at=$(date -Iseconds)"
  echo "host=$(hostname)"
  echo
  if command -v system_profiler >/dev/null 2>&1; then
    echo "[system_profiler]"
    system_profiler SPAudioDataType || true
  elif command -v pactl >/dev/null 2>&1; then
    echo "[pactl]"
    pactl list short sources || true
    pactl list short sinks || true
  elif command -v arecord >/dev/null 2>&1; then
    echo "[arecord]"
    arecord -l || true
    echo "[aplay]"
    aplay -l || true
  else
    echo "No supported audio inventory command found."
  fi
  echo
  echo "fallback_modes=wake_mode=vad_only,audio.scene=headset,audio.aec_enabled=false,audio.ns_enabled=false,perf.degraded_tts_voice=zh_CN-huayan-x_low"
} > "$OUT_FILE"
echo "$OUT_FILE"
