SELECT hash, ledger_sequence, account, operation_count, status, created_at
FROM stellar_benchmark.transactions
WHERE hash = {hash:String}
LIMIT 1;
