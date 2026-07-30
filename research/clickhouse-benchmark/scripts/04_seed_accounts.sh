#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck disable=SC1091
source "$ROOT/config.env"
# shellcheck disable=SC1091
source "$ROOT/scripts/_common.sh"

echo "seeding accounts from transactions (benchmark-only)..."

pg_psql -v ON_ERROR_STOP=1 <<'SQL'
INSERT INTO accounts (id, sequence, balance, last_modified_ledger, updated_at)
SELECT DISTINCT ON (account)
  account,
  account_sequence,
  0,
  ledger_sequence,
  created_at
FROM transactions
ORDER BY account, created_at DESC
ON CONFLICT (id) DO UPDATE SET
  sequence = EXCLUDED.sequence,
  last_modified_ledger = EXCLUDED.last_modified_ledger,
  updated_at = EXCLUDED.updated_at;
SQL

ch_client --multiquery <<'SQL'
TRUNCATE TABLE IF EXISTS stellar_benchmark.accounts;
INSERT INTO stellar_benchmark.accounts (id, sequence, balance, updated_at)
SELECT
  account,
  anyLast(account_sequence),
  '0',
  max(created_at)
FROM stellar_benchmark.transactions
GROUP BY account;
SQL

echo "account seed done"
