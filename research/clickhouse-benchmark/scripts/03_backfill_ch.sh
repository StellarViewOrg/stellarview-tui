#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
_OV_START="${START_LEDGER-}"
_OV_END="${END_LEDGER-}"
_OV_WORKERS="${WORKER_COUNT-}"
# shellcheck disable=SC1091
source "$ROOT/config.env"
[[ -n "${_OV_START}" ]] && START_LEDGER="$_OV_START"
[[ -n "${_OV_END}" ]] && END_LEDGER="$_OV_END"
[[ -n "${_OV_WORKERS}" ]] && WORKER_COUNT="$_OV_WORKERS"
REPO="$(cd "$ROOT/../.." && pwd)"
OUT_DIR="$ROOT/${RESULTS_DIR:-results}"
mkdir -p "$OUT_DIR"

START="${START_LEDGER}"
END="${END_LEDGER}"
WORKERS="${WORKER_COUNT}"

cd "$REPO/tui-indexer"
export CLICKHOUSE_DSN

go build -o bin/chbackfill ./cmd/chbackfill
BIN=./bin/chbackfill
[[ -f ./bin/chbackfill.exe ]] && BIN=./bin/chbackfill.exe

echo "clickhouse s3 backfill $START-$END workers=$WORKERS"
START_TS=$(date +%s)
set +e
"$BIN" --start "$START" --end "$END" --workers "$WORKERS" --dsn "$CLICKHOUSE_DSN"
RC=$?
set -e
END_TS=$(date +%s)
ELAPSED=$((END_TS - START_TS))
LEDGERS=$((END - START + 1))
RATE=$(awk -v n="$LEDGERS" -v s="$ELAPSED" 'BEGIN{ if (s<=0) s=1; printf "%.2f", n/s }')

{
  echo "engine=clickhouse"
  echo "start_ledger=$START"
  echo "end_ledger=$END"
  echo "ledgers=$LEDGERS"
  echo "workers=$WORKERS"
  echo "elapsed_sec=$ELAPSED"
  echo "ledgers_per_sec=$RATE"
  echo "exit_code=$RC"
} | tee "$OUT_DIR/throughput_ch.txt"

exit "$RC"
