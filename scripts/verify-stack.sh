#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

docker compose up --build -d

for _ in $(seq 1 30); do
  if curl -fsS --connect-timeout 1 --max-time 2 \
    http://127.0.0.1:18080/readyz >/dev/null; then
    break
  fi
  sleep 1
done

curl -fsS --connect-timeout 1 --max-time 2 \
  http://127.0.0.1:18080/healthz >/dev/null
curl -fsS --connect-timeout 1 --max-time 2 \
  http://127.0.0.1:18080/readyz >/dev/null
curl -fsS --connect-timeout 1 --max-time 2 \
  -H "Authorization: Bearer local-access-token" \
  http://127.0.0.1:18080/api/v1/watchlist >/dev/null

PROPULSE_E2E_BASE_URL=http://127.0.0.1:18080 \
PROPULSE_E2E_ACCESS_TOKEN=local-access-token \
go test ./internal/platform/app -run TestE2ESmoke -v
