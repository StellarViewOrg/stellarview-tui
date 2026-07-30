#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck disable=SC1091
source "$ROOT/config.env"
# shellcheck disable=SC1091
source "$ROOT/scripts/_common.sh"
OUT_DIR="$ROOT/${RESULTS_DIR:-results}"
mkdir -p "$OUT_DIR"
REPEATS="${QUERY_REPEATS:-11}"
OUT="$OUT_DIR/query_latency.txt"
: >"$OUT"
CH_HTTP="http://${CLICKHOUSE_HOST:-localhost}:${CLICKHOUSE_HTTP_PORT:-8123}/"

median_p95() {
  sort -n | awk '
    { a[NR]=$1 }
    END {
      if (NR==0) { print "nan,nan"; exit }
      mid = int((NR+1)/2)
      if (NR%2==1) med=a[mid]; else med=(a[mid]+a[mid+1])/2
      idx = int(0.95*(NR-1))+1
      if (idx<1) idx=1
      if (idx>NR) idx=NR
      printf "%.3f,%.3f", med, a[idx]
    }'
}

time_psql() {
  local label="$1"
  local sql="$2"
  local tmp ms stats line
  tmp=$(mktemp /tmp/pgbench.XXXXXX)
  for i in $(seq 1 "$REPEATS"); do
    line=$(pg_psql -v ON_ERROR_STOP=1 -q -c '\timing' -c "$sql" 2>&1 | tr -d '\r' | grep -E 'Time:' | tail -1 || true)
    ms=$(echo "$line" | awk '{for(i=1;i<=NF;i++) if($i=="Time:") print $(i+1)}')
    echo "${ms:-0}" >>"$tmp"
  done
  stats=$(median_p95 <"$tmp")
  echo "pg,$label,median_ms=${stats%,*},p95_ms=${stats#*,},repeats=$REPEATS" | tee -a "$OUT"
  rm -f "$tmp"
}

time_ch_http() {
  local label="$1"
  local sql="$2"
  local tmp start_ns end_ns ms stats
  tmp=$(mktemp /tmp/chbench.XXXXXX)
  for i in $(seq 1 "$REPEATS"); do
    start_ns=$(date +%s%N)
    curl -sS --get "$CH_HTTP" --data-urlencode "query=${sql}" >/dev/null
    end_ns=$(date +%s%N)
    ms=$(awk -v s="$start_ns" -v e="$end_ns" 'BEGIN{printf "%.3f", (e-s)/1000000}')
    echo "$ms" >>"$tmp"
  done
  stats=$(median_p95 <"$tmp")
  echo "ch,$label,median_ms=${stats%,*},p95_ms=${stats#*,},repeats=$REPEATS" | tee -a "$OUT"
  rm -f "$tmp"
}

HASH=$(pg_psql -At -c "SELECT hash FROM transactions ORDER BY created_at DESC LIMIT 1" | tr -d '\r')
ACCOUNT=$(pg_psql -At -c "SELECT id FROM accounts ORDER BY updated_at DESC LIMIT 1" | tr -d '\r')

if [[ -z "$HASH" || -z "$ACCOUNT" ]]; then
  echo "missing sample hash/account; did backfill and seed run?" >&2
  exit 1
fi

echo "sample_hash=$HASH" | tee "$OUT_DIR/query_samples.txt"
echo "sample_account=$ACCOUNT" | tee -a "$OUT_DIR/query_samples.txt"
echo "ch_transport=http_8123" | tee -a "$OUT_DIR/query_samples.txt"
echo "pg_transport=docker_exec_psql" | tee -a "$OUT_DIR/query_samples.txt"

time_psql "tx_by_hash" "SELECT hash, ledger_sequence, account, operation_count, status, created_at FROM transactions WHERE hash = '$HASH' LIMIT 1;"
time_psql "account_by_id" "SELECT id, sequence, balance::text, updated_at FROM accounts WHERE id = '$ACCOUNT' LIMIT 1;"
time_psql "account_history" "SELECT DISTINCT ON (t.hash) t.hash, t.ledger_sequence, t.account, t.operation_count, t.status, t.created_at FROM transactions t LEFT JOIN operations o ON o.transaction_hash = t.hash WHERE t.account = '$ACCOUNT' OR o.source_account = '$ACCOUNT' OR o.destination = '$ACCOUNT' ORDER BY t.hash, t.created_at DESC LIMIT 50 OFFSET 0;"
time_psql "counts_by_day_tx" "SELECT date_trunc('day', created_at) AS day, count(*) AS tx_count FROM transactions GROUP BY 1 ORDER BY 1;"
time_psql "counts_by_day_op" "SELECT date_trunc('day', created_at) AS day, count(*) AS op_count FROM operations GROUP BY 1 ORDER BY 1;"
time_psql "top_assets" "SELECT asset_code, asset_issuer, count(*) AS op_count FROM operations WHERE asset_code IS NOT NULL GROUP BY asset_code, asset_issuer ORDER BY op_count DESC LIMIT 20;"

time_ch_http "tx_by_hash" "SELECT hash, ledger_sequence, account, operation_count, status, created_at FROM stellar_benchmark.transactions WHERE hash = '$HASH' LIMIT 1"
time_ch_http "account_by_id" "SELECT id, sequence, balance, updated_at FROM stellar_benchmark.accounts WHERE id = '$ACCOUNT' LIMIT 1"
time_ch_http "account_history" "SELECT hash, ledger_sequence, account, operation_count, status, created_at FROM (SELECT t.hash AS hash, t.ledger_sequence AS ledger_sequence, t.account AS account, t.operation_count AS operation_count, t.status AS status, t.created_at AS created_at, row_number() OVER (PARTITION BY t.hash ORDER BY t.created_at DESC) AS rn FROM stellar_benchmark.transactions AS t LEFT JOIN stellar_benchmark.operations AS o ON o.transaction_hash = t.hash WHERE t.account = '$ACCOUNT' OR o.source_account = '$ACCOUNT' OR o.destination = '$ACCOUNT') WHERE rn = 1 ORDER BY created_at DESC LIMIT 50"
time_ch_http "counts_by_day_tx" "SELECT toStartOfDay(created_at) AS day, count() AS tx_count FROM stellar_benchmark.transactions GROUP BY day ORDER BY day"
time_ch_http "counts_by_day_op" "SELECT toStartOfDay(created_at) AS day, count() AS op_count FROM stellar_benchmark.operations GROUP BY day ORDER BY day"
time_ch_http "top_assets" "SELECT asset_code, asset_issuer, count() AS op_count FROM stellar_benchmark.operations WHERE asset_code IS NOT NULL GROUP BY asset_code, asset_issuer ORDER BY op_count DESC LIMIT 20"

echo "query measurements written"
