#!/usr/bin/env bash

set -euo pipefail

required=(FANGJIAN_AUTHORIZATION FANGJIAN_AK FANGJIAN_VERSION)
missing=()
for name in "${required[@]}"; do
  if [[ -z "${!name:-}" ]]; then
    missing+=("$name")
  fi
done
if ((${#missing[@]} > 0)); then
  printf 'missing required environment variables: %s\n' "${missing[*]}" >&2
  exit 1
fi

output="${1:-data/fangjian}"
community="${FANGJIAN_COMMUNITY:-all}"
exec go run ./cmd/fangjian-collector --output "$output" --community "$community"
