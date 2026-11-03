"""GROUP BY folds rows, a window keeps them. Run in SQLite.

Part 1: swaps per pool on 1 October, with WHERE, GROUP BY, HAVING and the COUNT(amount) trap.
Part 2: a running balance per address, ROW_NUMBER, the WHERE rn = 1 error, the CTE fix and ties.
Run: python3 main.py
"""
import sqlite3

SETUP = """
CREATE TABLE swaps (day TEXT, pool TEXT, amount INT);
INSERT INTO swaps VALUES ('30 Sep', 'p1', 10), ('1 Oct', 'p1', 10), ('1 Oct', 'p1', 20),
  ('1 Oct', 'p2', 5), ('1 Oct', 'p2', NULL), ('1 Oct', 'p3', 7);
CREATE TABLE ledger (address TEXT, ts TEXT, amount INT);
INSERT INTO ledger VALUES ('a', 't1', 10), ('a', 't2', 20), ('a', 't3', -5),
  ('b', 't1', 5), ('b', 't2', 5);
CREATE TABLE ties (address TEXT, ts TEXT, log_index INT);
INSERT INTO ties VALUES ('a', 't3', 0), ('a', 't3', 1), ('a', 't2', 0);
"""

PER_POOL = """
-- fold the rows of 1 October into one row per pool
SELECT pool, COUNT(*) AS swaps, SUM(amount) AS volume
FROM swaps
WHERE day = '1 Oct'
GROUP BY pool
ORDER BY pool;
"""

BUSY_POOLS = """
-- HAVING filters groups, after the fold
SELECT pool, COUNT(*) AS swaps, SUM(amount) AS volume
FROM swaps
WHERE day = '1 Oct'
GROUP BY pool
HAVING COUNT(*) > 1
ORDER BY pool;
"""

COUNT_TRAP = """
-- COUNT(amount) skips rows where amount is NULL
SELECT pool, COUNT(amount) AS swaps, SUM(amount) AS volume
FROM swaps
WHERE day = '1 Oct'
GROUP BY pool
HAVING COUNT(*) > 1
ORDER BY pool;
"""

RUNNING = """
-- every row stays, a window adds two columns
SELECT address, ts, amount,
       SUM(amount) OVER (PARTITION BY address ORDER BY ts) AS balance,
       ROW_NUMBER() OVER (PARTITION BY address ORDER BY ts DESC) AS rn
FROM ledger
ORDER BY address, ts;
"""

WHERE_RN = """
-- refused: WHERE runs before the window exists
SELECT address, ts,
       ROW_NUMBER() OVER (PARTITION BY address ORDER BY ts DESC) AS rn
FROM ledger
WHERE rn = 1;
"""

LATEST = """
-- the fix: name the window query, filter it outside
WITH r AS (
  SELECT address, ts, amount,
         SUM(amount) OVER (PARTITION BY address ORDER BY ts) AS balance,
         ROW_NUMBER() OVER (PARTITION BY address ORDER BY ts DESC) AS rn
  FROM ledger
)
SELECT address, ts, balance FROM r WHERE rn = 1 ORDER BY address;
"""

TIES = """
-- two rows at the same time t3: three ways to number them
SELECT ts, log_index,
       RANK() OVER (ORDER BY ts DESC) AS rnk,
       DENSE_RANK() OVER (ORDER BY ts DESC) AS dense,
       ROW_NUMBER() OVER (ORDER BY ts DESC) AS rn,
       ROW_NUMBER() OVER (ORDER BY ts DESC, log_index DESC) AS rn_fixed
FROM ties
ORDER BY ts DESC, log_index;
"""


def show(db, label, sql):
    print(label)
    for row in db.execute(sql):
        print("  ", row)


def main():
    db = sqlite3.connect(":memory:")
    db.executescript(SETUP)
    print("rows of 1 October:", db.execute("SELECT COUNT(*) FROM swaps WHERE day = '1 Oct'").fetchone()[0])
    show(db, "GROUP BY pool:", PER_POOL)
    show(db, "HAVING COUNT(*) > 1:", BUSY_POOLS)
    show(db, "COUNT(amount) instead:", COUNT_TRAP)
    print()
    show(db, "running balance and row number:", RUNNING)
    try:
        db.execute(WHERE_RN)
    except sqlite3.Error as e:
        print("WHERE rn = 1:", e)
    show(db, "latest row per address, via a CTE:", LATEST)
    show(db, "ties (ts, log_index, RANK, DENSE_RANK, ROW_NUMBER, with log_index):", TIES)


if __name__ == "__main__":
    main()
