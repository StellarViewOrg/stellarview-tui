#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck disable=SC1091
source "$ROOT/config.env"
# shellcheck disable=SC1091
source "$ROOT/scripts/_common.sh"

cd "$ROOT"
docker compose up -d
echo "waiting for clickhouse..."
for i in $(seq 1 60); do
  if ch_client --query "SELECT 1" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done
ch_client --multiquery < "$ROOT/schema/clickhouse/001_core.sql"
echo "clickhouse ready; schema applied"
