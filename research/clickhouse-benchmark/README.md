# ClickHouse vs TimescaleDB benchmark harness

Research spike only. Compares a fixed pubnet S3 backfill into:

1. The existing Postgres/TimescaleDB path (`tui-indexer s3backfill`)
2. A ClickHouse instance with an equivalent core schema (`tui-indexer chbackfill`)

No adopt / hybrid / stay decision lives here. Numbers and trade-offs are in
`docs/research/clickhouse-benchmark.md`.

## Fixed range

| Setting | Value |
|---------|-------|
| Start ledger | `55000000` |
| End ledger | `55010000` (inclusive) |
| Ledgers | 10001 |
| Workers | 8 |
| Network | pubnet S3 data lake |

Smoke range for harness checks: `55000000`–`55001000` (1001 ledgers).

## Prerequisites

- Docker
- Go 1.25+
- `infra/docker-compose.tui-indexer.yml` Postgres up (port `54330`)
- This directory's ClickHouse compose up (ports `8123` / `9000`)
- Network access to `s3://aws-public-blockchain/v1.1/stellar/ledgers/pubnet`

## How to run

From the repo root (Git Bash / WSL / Linux):

```bash
# 1) Postgres deps
docker compose -f infra/docker-compose.tui-indexer.yml up -d

# 2) ClickHouse + apply schema
cd research/clickhouse-benchmark
docker compose up -d
# wait until healthy, then:
./scripts/00_up.sh

# 3) Full recorded run (writes results/)
./scripts/07_run_all.sh

# Smoke only:
START_LEDGER=55000000 END_LEDGER=55001000 ./scripts/07_run_all.sh
```

Individual steps:

```bash
./scripts/01_reset.sh
./scripts/02_backfill_pg.sh
./scripts/03_backfill_ch.sh
./scripts/04_seed_accounts.sh
./scripts/05_measure_storage.sh
./scripts/06_measure_queries.sh
```

## Layout

| Path | Role |
|------|------|
| `config.env` | Fixed range and connection defaults |
| `schema/clickhouse/` | ClickHouse DDL |
| `queries/` | Matched PG and CH SQL |
| `scripts/` | Orchestration |
| `results/` | Raw timings from the recorded run |
| `../../tui-indexer/cmd/chbackfill` | S3 -> ClickHouse ingest binary |

## Notes

- Core tables measured: `ledgers`, `transactions`, `operations`, `contract_events`.
- S3 backfill does not populate production `accounts`. Script `04_seed_accounts.sh`
  seeds a minimal accounts table from distinct tx accounts for point-lookup timing only.
- `chbackfill` lives under the `tui-indexer` module so it can reuse `internal/` packages.
