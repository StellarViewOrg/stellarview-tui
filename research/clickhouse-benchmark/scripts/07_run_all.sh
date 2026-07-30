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
export START_LEDGER END_LEDGER WORKER_COUNT
OUT_DIR="$ROOT/${RESULTS_DIR:-results}"
mkdir -p "$OUT_DIR"

HOST_INFO="$OUT_DIR/host_info.txt"
{
  echo "date_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "uname=$(uname -a 2>/dev/null || true)"
  echo "start_ledger=$START_LEDGER"
  echo "end_ledger=$END_LEDGER"
  echo "workers=$WORKER_COUNT"
  if command -v nproc >/dev/null 2>&1; then
    echo "nproc=$(nproc)"
  fi
  if [[ -r /proc/meminfo ]]; then
    awk '/MemTotal/{print "mem_total_kb="$2}' /proc/meminfo
  fi
} | tee "$HOST_INFO"

"$ROOT/scripts/00_up.sh"
"$ROOT/scripts/01_reset.sh"
"$ROOT/scripts/02_backfill_pg.sh"
"$ROOT/scripts/03_backfill_ch.sh"
"$ROOT/scripts/04_seed_accounts.sh"
"$ROOT/scripts/05_measure_storage.sh"
"$ROOT/scripts/06_measure_queries.sh"

echo "run complete; artifacts in $OUT_DIR"
