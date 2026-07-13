#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

docker compose up --build -d

curl -fsS http://127.0.0.1:8317/healthz >/dev/null
curl -fsS http://127.0.0.1:8317/api/v1/watchlist >/dev/null

PROPULSE_E2E_BASE_URL=http://127.0.0.1:8317 go test ./internal/platform/app -run TestE2ESmoke -v
