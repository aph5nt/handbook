"""Recursive CTEs and subqueries. Run in SQLite.

Tab 1: a money walk from address S, three hops, a depth bound, and the cycle with no bound.
Tab 2: an uncorrelated subquery runs once, a correlated one runs once per row, a grouped CTE fixes it.
Run: python3 main.py
"""
import sqlite3

SETUP = """
CREATE TABLE transfers (from_address TEXT, to_address TEXT);
INSERT INTO transfers VALUES ('S', 'A'), ('S', 'B'), ('A', 'C'), ('B', 'D'), ('D', 'E'), ('C', 'S');
CREATE TABLE swaps (pool TEXT, amount INT);
INSERT INTO swaps VALUES ('p1', 120), ('p1', 40), ('p2', 5), ('p2', 9);
CREATE TABLE pools (pool TEXT, dex TEXT);
INSERT INTO pools VALUES ('p1', 'aerodrome'), ('p2', 'uniswap');
"""

WALK = """
-- walk the money out of S, at most three hops
WITH RECURSIVE hops(to_address, depth) AS (
  SELECT to_address, 1 FROM transfers WHERE from_address = 'S'
  UNION ALL
  SELECT t.to_address, h.depth + 1
  FROM hops h JOIN transfers t ON t.from_address = h.to_address
  WHERE h.depth < 3
)
SELECT to_address, depth FROM hops ORDER BY depth, to_address;
"""

NO_BOUND = """
-- the same walk with no bound: the cycle C -> S feeds it for ever
WITH RECURSIVE hops(to_address, depth) AS (
  SELECT to_address, 1 FROM transfers WHERE from_address = 'S'
  UNION ALL
  SELECT t.to_address, h.depth + 1
  FROM hops h JOIN transfers t ON t.from_address = h.to_address
)
SELECT to_address, depth FROM hops LIMIT 20;
"""

IN_LIST = """
-- the inner query names nothing outside itself: it runs once
SELECT pool, amount FROM swaps
WHERE pool IN (SELECT pool FROM pools WHERE dex = 'aerodrome');
"""

CORRELATED = """
-- the inner query names s.pool: it runs once per outer row
-- (inner_run() counts each run and returns 1)
SELECT pool, amount FROM swaps s
WHERE amount > (SELECT AVG(amount) * inner_run() FROM swaps s2
                WHERE s2.pool = s.pool);
"""

GROUPED_FIX = """
-- the fix: every average in one scan, then one join
WITH avg_per_pool AS (
  SELECT pool, AVG(amount) AS a FROM swaps GROUP BY pool
)
SELECT s.pool, s.amount FROM swaps s
JOIN avg_per_pool p ON p.pool = s.pool
WHERE s.amount > p.a;
"""


def main():
    db = sqlite3.connect(":memory:")
    runs = [0]

    def inner_run():
        runs[0] += 1
        return 1

    db.create_function("inner_run", 0, inner_run)
    db.executescript(SETUP)

    print("walk with depth < 3:", db.execute(WALK).fetchall())
    print("distinct addresses reached:", sorted({r[0] for r in db.execute(WALK)}))
    rows = db.execute(NO_BOUND).fetchall()
    print("no bound, first 20 rows:", rows)
    print("no bound: depth", max(d for _, d in rows), "after 20 rows and still going (LIMIT 20 cut it)")
    print()

    print("uncorrelated:", db.execute(IN_LIST).fetchall())
    print("  plan:", [r[3] for r in db.execute("EXPLAIN QUERY PLAN " + IN_LIST)])
    print("averages per pool:", db.execute("SELECT pool, AVG(amount) FROM swaps GROUP BY pool").fetchall())
    print("correlated:", db.execute(CORRELATED).fetchall(), "inner runs:", runs[0])
    print("  plan:", [r[3] for r in db.execute("EXPLAIN QUERY PLAN " + CORRELATED)])
    print("grouped CTE fix:", db.execute(GROUPED_FIX).fetchall())


if __name__ == "__main__":
    main()
