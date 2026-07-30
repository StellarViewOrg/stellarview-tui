SELECT DISTINCT ON (t.hash)
    t.hash,
    t.ledger_sequence,
    t.account,
    t.operation_count,
    t.status,
    t.created_at
FROM transactions t
LEFT JOIN operations o ON o.transaction_hash = t.hash
WHERE t.account = :account
   OR o.source_account = :account
   OR o.destination = :account
ORDER BY t.hash, t.created_at DESC
LIMIT 50 OFFSET 0;
