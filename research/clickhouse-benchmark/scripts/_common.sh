#!/usr/bin/env bash
# Shared helpers for benchmark scripts.
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
BENCH_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PG_COMPOSE="$REPO_ROOT/infra/docker-compose.tui-indexer.yml"
CH_COMPOSE="$BENCH_ROOT/docker-compose.yml"

pg_psql() {
  docker compose -f "$PG_COMPOSE" exec -T postgres-tui \
    psql -U explorer -d stellar_explorer_tui "$@"
}

ch_client() {
  docker compose -f "$CH_COMPOSE" exec -T clickhouse clickhouse-client "$@"
}
