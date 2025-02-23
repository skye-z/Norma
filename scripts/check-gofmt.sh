#!/usr/bin/env bash
set -euo pipefail
ROOT="${1:-.}"
mapfile -t files < <(find "$ROOT" -type f -name '*.go' ! -path '*/.gocache/*' ! -path '*/.gomodcache/*')
if [[ ${#files[@]} -eq 0 ]]; then
  echo '[OK] no go files found'
  exit 0
fi
listed="$(gofmt -l "${files[@]}")"
if [[ -n "$listed" ]]; then
  echo '[FAIL] gofmt required for:' >&2
  echo "$listed" >&2
  exit 1
fi
echo '[OK] gofmt check passed'