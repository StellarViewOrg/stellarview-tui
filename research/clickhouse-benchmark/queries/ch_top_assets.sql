SELECT asset_code, asset_issuer, count() AS op_count
FROM stellar_benchmark.operations
WHERE asset_code IS NOT NULL
GROUP BY asset_code, asset_issuer
ORDER BY op_count DESC
LIMIT 20;
