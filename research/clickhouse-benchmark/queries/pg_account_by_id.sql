SELECT id, sequence, balance::text, updated_at
FROM accounts
WHERE id = :id
LIMIT 1;
