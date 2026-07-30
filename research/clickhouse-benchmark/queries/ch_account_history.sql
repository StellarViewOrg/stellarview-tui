SELECT
    hash,
    ledger_sequence,
    account,
    operation_count,
    status,
    created_at
FROM (
    SELECT
        t.hash AS hash,
        t.ledger_sequence AS ledger_sequence,
        t.account AS account,
        t.operation_count AS operation_count,
        t.status AS status,
        t.created_at AS created_at,
        row_number() OVER (PARTITION BY t.hash ORDER BY t.created_at DESC) AS rn
    FROM stellar_benchmark.transactions AS t
    LEFT JOIN stellar_benchmark.operations AS o ON o.transaction_hash = t.hash
    WHERE t.account = {account:String}
       OR o.source_account = {account:String}
       OR o.destination = {account:String}
)
WHERE rn = 1
ORDER BY created_at DESC
LIMIT 50;
