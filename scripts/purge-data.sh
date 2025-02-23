#!/usr/bin/env bash
set -euo pipefail
API_BASE="${1:-http://127.0.0.1:8090}"
curl -sS -X POST -H 'Content-Type: application/json' -d '{}' "${API_BASE%/}/privacy/purge"
