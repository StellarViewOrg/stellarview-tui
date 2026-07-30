-- Point lookup: transaction by hash
-- :hash replaced by measure script
SELECT hash, ledger_sequence, account, operation_count, status, created_at
FROM transactions
WHERE hash = :hash
LIMIT 1;
