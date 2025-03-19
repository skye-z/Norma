#!/usr/bin/env bash
set -euo pipefail
ROOT="${1:-.}"
CONFIG_PATH="${2:-config.yaml}"
cd "$ROOT"

export GOCACHE="${PWD}/.gocache"
export GOMODCACHE="${PWD}/.gomodcache"
mkdir -p "$GOCACHE" "$GOMODCACHE"

RUN_ROOT="${PWD}/resource/diagnostics/alpha-smoke"
LOG_DIR="${RUN_ROOT}/logs"
OUTPUT_LOG="${RUN_ROOT}/demo-output.log"
rm -rf "$RUN_ROOT"
mkdir -p "$LOG_DIR"

export NORMA_APP_LOG_DIR="$LOG_DIR"
export NORMA_LLM_PROVIDER="mock"
export NORMA_ASR_PROVIDER="mock"
export NORMA_TTS_PROVIDER="mock"
export NORMA_AUDIO_WAKE_MODE="vad"

echo '[alpha] step 1/3: focused voice tests'
go test ./internal/audio -run 'TestPipelineUsesFrameASR|TestPipelineSupportsWakeWordMode' -count=1
go test ./internal/runtime -run 'TestRegressionVoiceQAFlow|TestRegressionVoiceInterrupt' -count=1

echo '[alpha] step 2/3: demo voice chain run'
go run . -config "$CONFIG_PATH" -demo 2>&1 | tee "$OUTPUT_LOG"

echo '[alpha] step 3/3: verify logs'
APP_LOG="${LOG_DIR}/app.log"
[[ -f "$APP_LOG" ]] || { echo "app log missing: $APP_LOG" >&2; exit 1; }
for needle in \
  '[demo] asr.partial=' \
  '[runtime] asr.final -> Thinking' \
  '[demo] llm.token=' \
  '[demo] tts.chunk bytes=' \
  '[metric] summary' \
  '[demo] final state=Listening'
do
  grep -F "$needle" "$APP_LOG" >/dev/null || { echo "missing expected log fragment: $needle" >&2; exit 1; }
done

echo "[OK] v0.3 Alpha acceptance passed"
echo "[OK] logs: $APP_LOG"