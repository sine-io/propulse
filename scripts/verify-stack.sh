#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

sudo docker compose up --build -d

API_ADDRESS="$(sudo docker compose port propulse 8080)"
BASE_URL="http://${API_ADDRESS}"
ACCESS_TOKEN="$(sudo docker compose exec -T propulse printenv PROPULSE_ACCESS_TOKEN | tr -d '\r\n')"

for _ in $(seq 1 30); do
  if curl -fsS --connect-timeout 1 --max-time 2 \
    "${BASE_URL}/readyz" >/dev/null; then
    break
  fi
  sleep 1
done

curl -fsS --connect-timeout 1 --max-time 2 \
  "${BASE_URL}/healthz" >/dev/null
curl -fsS --connect-timeout 1 --max-time 2 \
  "${BASE_URL}/readyz" >/dev/null
curl -fsS --connect-timeout 1 --max-time 2 \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  "${BASE_URL}/api/v1/watchlist" >/dev/null

PROPULSE_E2E_BASE_URL="${BASE_URL}" \
PROPULSE_E2E_ACCESS_TOKEN="${ACCESS_TOKEN}" \
go test ./internal/platform/app -run TestE2ESmoke -v
