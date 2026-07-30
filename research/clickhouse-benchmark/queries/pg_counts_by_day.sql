SELECT date_trunc('day', created_at) AS day,
       count(*) AS tx_count
FROM transactions
GROUP BY 1
ORDER BY 1;

SELECT date_trunc('day', created_at) AS day,
       count(*) AS op_count
FROM operations
GROUP BY 1
ORDER BY 1;
