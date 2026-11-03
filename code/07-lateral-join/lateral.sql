-- PostgreSQL: each pool with its 3 newest swaps
CREATE INDEX swaps_pool_ts ON swaps (pool, ts DESC);

SELECT p.pool, x.ts, x.amount
FROM pools p
LEFT JOIN LATERAL (
  SELECT ts, amount FROM swaps s
  WHERE s.pool = p.pool ORDER BY ts DESC LIMIT 3
) x ON true;

-- PostgreSQL without LATERAL, or a warehouse: ROW_NUMBER, one pass
WITH r AS (
  SELECT pool, ts, amount,
         ROW_NUMBER() OVER (PARTITION BY pool ORDER BY ts DESC) AS rn
  FROM swaps
)
SELECT p.pool, r.ts, r.amount
FROM pools p LEFT JOIN r ON r.pool = p.pool AND r.rn <= 3;
