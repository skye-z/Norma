#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EMBEDDED_TAGS="${EMBEDDED_TAGS:-whisper_embedded sherpa_embedded}"

"$SCRIPT_DIR/embedded-env-mingw64.sh" go test -tags "$EMBEDDED_TAGS" ./internal/audio
"$SCRIPT_DIR/embedded-env-mingw64.sh" go test -tags "$EMBEDDED_TAGS" ./...

printf "[embedded-build] done: embedded build + tests passed\n"
