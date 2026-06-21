#!/usr/bin/env bash
# Run Cylon backend + frontend with hot reload for local development.
#
# Starts (1) mock-lns, (2) cylon serve wired to it, and (3) the Vite dev server
# which proxies /api and /ws to the backend. Requires Go, Node, and an AppKey
# shared between mock-lns and any tags you create.
set -euo pipefail
cd "$(dirname "$0")/.."

APP_KEY="${CYLON_APP_KEY:-000102030405060708090a0b0c0d0e0f}"
export CYLON_GATEWAY_LNS_URL="${CYLON_GATEWAY_LNS_URL:-ws://127.0.0.1:7000}"
export CYLON_STORE_PATH="${CYLON_STORE_PATH:-./cylon-dev.db}"

pids=()
cleanup() { kill "${pids[@]}" 2>/dev/null || true; }
trap cleanup EXIT

echo "▶ mock-lns on :7000"
go run ./cmd/mock-lns --listen 127.0.0.1:7000 --app-key "$APP_KEY" &
pids+=($!)

sleep 1
echo "▶ cylon serve on :8080 (API/WS)"
go run ./cmd/cylon serve &
pids+=($!)

echo "▶ vite dev server (proxying to :8080)"
cd web && npm run dev
