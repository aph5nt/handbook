"""Top 3 newest swaps per pool, run in SQLite.

SQLite has no LATERAL. This program does what LATERAL does by hand: it runs
the same subquery once per pool, with that pool's id. Then it shows the plan
with and without an index on (pool, ts), and the ROW_NUMBER version that
needs no LATERAL. The PostgreSQL LATERAL query is in lateral.sql.
Run: python3 main.py
"""
import sqlite3

SETUP = """
CREATE TABLE pools (pool TEXT);
CREATE TABLE swaps (pool TEXT, ts TEXT, amount INT);
INSERT INTO pools VALUES ('p1'), ('p2'), ('p3');
INSERT INTO swaps VALUES ('p1', '10:05', 40), ('p1', '10:02', 15), ('p1', '09:58', 22), ('p1', '09:30', 5);
INSERT INTO swaps VALUES ('p2', '10:01', 8), ('p2', '09:40', 30);
"""

POOLS = """
-- the left side: one row per pool
SELECT pool FROM pools ORDER BY pool;
"""

NEWEST_3 = """
-- the subquery, run once per pool with that pool's id
SELECT ts, amount FROM swaps
WHERE pool = ? ORDER BY ts DESC LIMIT 3;
"""

INDEX = """
-- the index the subquery needs: its filter, then its sort
CREATE INDEX swaps_pool_ts ON swaps (pool, ts DESC);
"""

TOP3_ROW_NUMBER = """
-- no LATERAL: number each pool's swaps newest first, keep 3
WITH r AS (
  SELECT pool, ts, amount,
         ROW_NUMBER() OVER (PARTITION BY pool ORDER BY ts DESC) AS rn
  FROM swaps
)
SELECT p.pool, r.ts, r.amount
FROM pools p LEFT JOIN r ON r.pool = p.pool AND r.rn <= 3
ORDER BY p.pool, r.ts DESC;
"""


def lateral(db, keep_empty):
    """JOIN LATERAL by hand. keep_empty=True is LEFT JOIN LATERAL ... ON true."""
    out = []
    for (pool,) in db.execute(POOLS).fetchall():
        rows = db.execute(NEWEST_3, (pool,)).fetchall()
        if not rows and keep_empty:
            out.append((pool, None, None))
        for ts, amount in rows:
            out.append((pool, ts, amount))
    return out


def plan(db):
    return " / ".join(r[3] for r in db.execute("EXPLAIN QUERY PLAN " + NEWEST_3, ("p1",)))


def show(rows):
    return [f"{p} {'NULL' if t is None else t} {'NULL' if a is None else a}" for p, t, a in rows]


def main():
    db = sqlite3.connect(":memory:")
    db.executescript(SETUP)
    db.executescript(INDEX)
    for (pool,) in db.execute(POOLS).fetchall():
        print(f"run for {pool}: {db.execute(NEWEST_3, (pool,)).fetchall()}")
    rows = lateral(db, keep_empty=False)
    print(f"JOIN LATERAL: {len(rows)} rows {show(rows)}")
    rows = lateral(db, keep_empty=True)
    print(f"LEFT JOIN LATERAL: {len(rows)} rows {show(rows)}")
    rows = db.execute(TOP3_ROW_NUMBER).fetchall()
    print(f"ROW_NUMBER, rn <= 3: {len(rows)} rows {show(rows)}")
    print()

    big = sqlite3.connect(":memory:")
    big.executescript("CREATE TABLE pools (pool TEXT); CREATE TABLE swaps (pool TEXT, ts TEXT, amount INT);")
    n_pools, per_pool = 1000, 50
    big.executemany("INSERT INTO pools VALUES (?)", [(f"p{k}",) for k in range(n_pools)])
    big.executemany("INSERT INTO swaps VALUES (?, ?, ?)",
                    [(f"p{k}", f"{m:04d}", m) for k in range(n_pools) for m in range(per_pool)])
    n_swaps = big.execute("SELECT COUNT(*) FROM swaps").fetchone()[0]
    print(f"{n_pools:,} pools, {n_swaps:,} swaps")
    print(f"plan, no index: {plan(big)}")
    print(f"rows read, no index: {n_pools:,} runs x {n_swaps:,} = {n_pools * n_swaps:,}")
    big.executescript(INDEX)
    print(f"plan, with index: {plan(big)}")
    rows = lateral(big, keep_empty=True)
    print(f"index entries read, with index: {len(rows):,} (3 per pool)")


if __name__ == "__main__":
    main()
