SELECT toStartOfDay(created_at) AS day, count() AS tx_count
FROM stellar_benchmark.transactions
GROUP BY day
ORDER BY day;

SELECT toStartOfDay(created_at) AS day, count() AS op_count
FROM stellar_benchmark.operations
GROUP BY day
ORDER BY day;
