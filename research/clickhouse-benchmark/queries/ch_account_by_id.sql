SELECT id, sequence, balance, updated_at
FROM stellar_benchmark.accounts
WHERE id = {id:String}
LIMIT 1;
