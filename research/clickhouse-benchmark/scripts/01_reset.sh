#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck disable=SC1091
source "$ROOT/config.env"
# shellcheck disable=SC1091
source "$ROOT/scripts/_common.sh"

echo "resetting ClickHouse database..."
ch_client --query "DROP DATABASE IF EXISTS stellar_benchmark"
ch_client --multiquery < "$ROOT/schema/clickhouse/001_core.sql"

echo "resetting Postgres core tables (truncate)..."
pg_psql -v ON_ERROR_STOP=1 <<'SQL'
TRUNCATE TABLE contract_events, token_events, operations, transactions, ledgers, accounts CASCADE;
DELETE FROM ingestion_state;
SQL

echo "reset complete"
