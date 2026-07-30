#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck disable=SC1091
source "$ROOT/config.env"
# shellcheck disable=SC1091
source "$ROOT/scripts/_common.sh"
OUT_DIR="$ROOT/${RESULTS_DIR:-results}"
mkdir -p "$OUT_DIR"

{
  echo "# postgres hypertable and table sizes (bytes)"
  pg_psql -At -F',' -c "
SELECT hypertable_name,
       hypertable_size(format('%I.%I', hypertable_schema, hypertable_name)::regclass) AS total_bytes
FROM timescaledb_information.hypertables
WHERE hypertable_name IN ('ledgers','transactions','operations','contract_events')
ORDER BY hypertable_name;
"
  pg_psql -At -F',' -c "
SELECT 'accounts',
       pg_total_relation_size('accounts'::regclass),
       pg_relation_size('accounts'::regclass),
       pg_indexes_size('accounts'::regclass);
"
  echo "# postgres row counts"
  pg_psql -At -F',' -c "
SELECT 'ledgers', count(*) FROM ledgers
UNION ALL SELECT 'transactions', count(*) FROM transactions
UNION ALL SELECT 'operations', count(*) FROM operations
UNION ALL SELECT 'contract_events', count(*) FROM contract_events
UNION ALL SELECT 'accounts', count(*) FROM accounts;
"
  echo "# timescale compression stats (may be empty for recent chunks)"
  set +e
  pg_psql -At -F',' -c "
SELECT hypertable_name, total_chunks, number_compressed_chunks,
       before_compression_total_bytes, after_compression_total_bytes
FROM timescaledb_information.hypertable_compression_stats
WHERE hypertable_name IN ('ledgers','transactions','operations','contract_events');
"
  if [[ $? -ne 0 ]]; then
    echo "compression_stats_unavailable"
  fi
  set -e
} | tee "$OUT_DIR/storage_pg.txt"

{
  echo "# clickhouse parts"
  ch_client -q "
SELECT
  table,
  sum(rows) AS row_count,
  sum(bytes_on_disk) AS on_disk_bytes,
  sum(data_uncompressed_bytes) AS uncompressed_bytes,
  if(sum(bytes_on_disk) = 0, 0., sum(data_uncompressed_bytes) / sum(bytes_on_disk)) AS compression_ratio
FROM system.parts
WHERE database = 'stellar_benchmark'
  AND active
  AND table IN ('ledgers','transactions','operations','contract_events','accounts')
GROUP BY table
ORDER BY table
FORMAT TSVWithNames
"
  echo "# clickhouse row counts"
  ch_client -q "
SELECT 'ledgers', count() FROM stellar_benchmark.ledgers
UNION ALL SELECT 'transactions', count() FROM stellar_benchmark.transactions
UNION ALL SELECT 'operations', count() FROM stellar_benchmark.operations
UNION ALL SELECT 'contract_events', count() FROM stellar_benchmark.contract_events
UNION ALL SELECT 'accounts', count() FROM stellar_benchmark.accounts
FORMAT TSV
"
} | tee "$OUT_DIR/storage_ch.txt"

echo "storage measurements written"
