# ClickHouse vs TimescaleDB backfill benchmark

Research spike only. This document records measured numbers and observed
trade-offs for a fixed pubnet S3 backfill into both stores. It does not make
an adopt / hybrid / stay decision.

Harness: `research/clickhouse-benchmark/`

## Setup and fixed range

| Item | Value |
|------|-------|
| Start ledger | 55000000 |
| End ledger | 55010000 (inclusive) |
| Ledgers | 10001 |
| Workers | 8 |
| Source | pubnet S3 data lake (`aws-public-blockchain/.../pubnet`) |
| Postgres | TimescaleDB pg16 via `infra/docker-compose.tui-indexer.yml` |
| ClickHouse | `clickhouse/clickhouse-server:24.8` via harness compose |
| Host | Windows MSYS, 16 logical CPUs, ~15.2 GiB RAM |
| Recorded at | 2026-07-30T06:01:18Z (host_info) |

Core tables compared: `ledgers`, `transactions`, `operations`, `contract_events`.

`accounts` was seeded after ingest from distinct transaction accounts. The S3
path does not populate production accounts; the seed exists only for
account-by-id timing.

## Method

Held constant:

- Same ledger range and worker count
- Same S3 source (anonymous pubnet data lake)
- Same transform helpers from `tui-indexer` for ledger / tx / op / contract event shaping
- Query shapes aligned to explorer access patterns plus the aggregates asked for in the issue

Differences that matter:

- Postgres ingest uses the existing `s3backfill` + `ProcessOneLedger` path
- ClickHouse ingest uses `tui-indexer/cmd/chbackfill` (same S3 read, native CH batch inserts)
- Query transport: Postgres timings use `psql` inside the container; ClickHouse
  timings use HTTP on host port 8123 (avoids `docker exec` overhead)
- Timescale compression policies did not compress these recent chunks
  (`hypertable_compression_stats` unavailable / empty on this image)

Raw artifacts: `research/clickhouse-benchmark/results/`

## Storage results

Row counts:

| Table | Postgres | ClickHouse |
|-------|----------|------------|
| ledgers | 10001 | 10001 |
| transactions | 2429520 | 2429520 |
| operations | 4788835 | 4788835 |
| contract_events | 2815553 | 1192534 |
| accounts (seed) | 59253 | 59253 |

`contract_events` diverged. Ledgers / transactions / operations matched exactly.
The ClickHouse table used `ReplacingMergeTree` ordered by
`(contract_id, created_at, transaction_hash)`. Multiple events in one transaction
often share that key, so merges can collapse rows. The harness schema was later
widened to include topics and value in the sort key; the recorded run above is
from before a full re-ingest with that fix. Treat event storage as not
apples-to-apples for this run.

Postgres sizes (`hypertable_size` / `pg_total_relation_size`):

| Table | Bytes on disk |
|-------|---------------|
| ledgers | 11,993,088 |
| transactions | 6,301,810,688 |
| operations | 2,911,141,888 |
| contract_events | 3,016,482,816 |
| accounts | 32,776,192 |
| Core four total | 12,241,428,480 (~11.40 GiB) |

ClickHouse active parts:

| Table | On disk | Uncompressed | Ratio (uncomp/on_disk) |
|-------|---------|--------------|------------------------|
| ledgers | 4,703,260 | 8,230,823 | 1.75 |
| transactions | 3,475,988,741 | 9,026,282,345 | 2.60 |
| operations | 441,522,747 | 1,587,515,243 | 3.60 |
| contract_events | 164,676,938 | 960,447,283 | 5.83 |
| accounts | 3,954,262 | 4,917,999 | 1.24 |
| Core four total on disk | 4,086,891,686 (~3.81 GiB) | | |

Notes:

- ClickHouse on-disk footprint for the four core tables was about 3.0x smaller
  than Postgres on this run, even with fewer contract_events rows on CH.
- Much of the Postgres size is wide XDR text columns plus btree indexes.
- Timescale compression did not apply to these chunks yet, so Postgres numbers
  are uncompressed-chunk sizes.

## Ingestion throughput

| Engine | Elapsed (s) | Ledgers/sec |
|--------|-------------|-------------|
| Postgres (`s3backfill`) | 2984 | 3.35 |
| ClickHouse (`chbackfill`) | 351 | 28.49 |

Same machine, same range, same worker count. Both spent time on S3 downloads.
ClickHouse finished about 8.5x faster wall clock. Possible reasons observed:

- Batch native inserts vs per-ledger Postgres transactions
- Less index maintenance during write
- Worker-local CH connections after fixing an early shared-connection race

Throughput figures exclude account seeding.

## Query latency

Median / p95 over 11 repeats. Warm-ish process caches; first runs not isolated
as cold-cache OS page cache flushes.

| Query | Postgres median (ms) | Postgres p95 | ClickHouse median (ms) | ClickHouse p95 |
|-------|----------------------|--------------|------------------------|----------------|
| tx_by_hash | 7.6 | 8.4 | 71.2 | 95.7 |
| account_by_id | 6.8 | 8.1 | 65.2 | 76.2 |
| account_history (limit 50) | 398.2 | 419.2 | 1636.4 | 2064.9 |
| counts_by_day (txs) | 179.4 | 220.8 | 82.9 | 85.8 |
| counts_by_day (ops) | 261.4 | 288.7 | 82.2 | 96.6 |
| top_assets | 222.8 | 224.9 | 146.7 | 152.3 |

Sample keys are in `results/query_samples.txt`.

## Observed trade-offs

### Point lookups

Postgres was faster for hash and account primary-key style lookups on this
dataset and schema. ClickHouse `ORDER BY (hash)` still paid more latency per
request over HTTP than indexed Postgres. For an explorer UI that does many
single-entity fetches, that gap is material.

### Account history

Both sides ran a join-heavy history shape. Postgres stayed under ~0.4s median.
ClickHouse was several times slower here with the chosen order keys and no
projection enabled (projections were dropped because they conflicted with
`ReplacingMergeTree` defaults on 24.8).

### Aggregates

Day counts and top assets favored ClickHouse (lower median than Postgres).
That matches columnar scan behavior on this workload size.

### Dedup model

Postgres ingest uses `ON CONFLICT DO NOTHING` on ledgers / txs / ops (immediate
idempotency). ClickHouse used `ReplacingMergeTree(ingested_at)`, which dedups
asynchronously during merges. That is not the same as an upsert. Event rows that
share a coarse sort key can disappear until the sort key is made unique enough.
Anyone comparing engines needs an explicit dedup story, not a silent assumption
that CH inserts behave like Postgres conflicts.

### Operational complexity

- Postgres path already exists in production code and CI.
- ClickHouse adds another server image, schema dialect, client library, and
  backup/ops surface.
- Local harness needed separate compose, HTTP query transport, and a dedicated
  ingest binary under the indexer module (Go `internal/` packages cannot be
  imported from a separate module).

### Dev ergonomics

- Reusing `transform` / `source` kept row shapes close for txs and ops.
- SQL still diverged (`DISTINCT ON` vs window functions, `date_trunc` vs
  `toStartOfDay`).
- Measuring fairly required care: `docker exec` around `clickhouse-client`
  added hundreds of milliseconds and was abandoned for HTTP.

### S3 path limits

Token events and new contract upserts are largely absent on S3 backfill (empty
`MetadataXDR`, no RPC). This experiment measures the tables that path actually
fills, plus a synthetic accounts seed for lookup timing.

## Raw artifacts

Under `research/clickhouse-benchmark/results/`:

- `host_info.txt`
- `throughput_pg.txt` / `throughput_ch.txt`
- `storage_pg.txt` / `storage_ch.txt`
- `query_latency.txt` / `query_samples.txt`

How to reproduce: see `research/clickhouse-benchmark/README.md` and
`./scripts/07_run_all.sh`.
