#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# Preserve caller overrides (config.env would otherwise clobber them).
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
export DATABASE_URL
export WORKER_COUNT="$WORKERS"

if [[ ! -x ./bin/tui-indexer && ! -f ./bin/tui-indexer.exe ]]; then
  go build -o bin/tui-indexer ./cmd/tui-indexer
fi
BIN=./bin/tui-indexer
[[ -f ./bin/tui-indexer.exe ]] && BIN=./bin/tui-indexer.exe

echo "ensuring migrations..."
"$BIN" migrate

echo "postgres s3backfill $START-$END workers=$WORKERS"
START_TS=$(date +%s)
set +e
"$BIN" s3backfill --start "$START" --end "$END"
RC=$?
set -e
END_TS=$(date +%s)
ELAPSED=$((END_TS - START_TS))
LEDGERS=$((END - START + 1))
RATE=$(awk -v n="$LEDGERS" -v s="$ELAPSED" 'BEGIN{ if (s<=0) s=1; printf "%.2f", n/s }')

{
  echo "engine=postgres"
  echo "start_ledger=$START"
  echo "end_ledger=$END"
  echo "ledgers=$LEDGERS"
  echo "workers=$WORKERS"
  echo "elapsed_sec=$ELAPSED"
  echo "ledgers_per_sec=$RATE"
  echo "exit_code=$RC"
} | tee "$OUT_DIR/throughput_pg.txt"

exit "$RC"
