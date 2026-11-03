"""NULL and three-valued logic, run in SQLite.

Part 1: WHERE price > 0 drops the two transfers whose price is NULL.
Part 2: NOT IN with a NULL in the list returns zero rows. NOT EXISTS does not.
Run: python3 main.py
"""
import sqlite3

SETUP = """
CREATE TABLE transfers (tx TEXT, price REAL);
INSERT INTO transfers VALUES ('t1', 2.5), ('t2', NULL), ('t3', 4.0), ('t4', NULL);
CREATE TABLE swaps (pool TEXT);
CREATE TABLE banned (pool TEXT);
INSERT INTO swaps VALUES ('p1'), ('p2'), ('p3');
INSERT INTO banned VALUES ('p2'), (NULL);
"""

TRUTH = """
-- what price > 0 gives for each row: 1 TRUE, 0 FALSE, NULL UNKNOWN
SELECT tx, price, price > 0 FROM transfers;
"""

PRICED = """
-- the report: WHERE keeps only TRUE
SELECT tx, price FROM transfers
WHERE price > 0;
"""

NULL_COMPARE = """
-- three comparisons with a NULL
SELECT NULL = NULL, NULL <> 5, NULL IS NULL;
"""

UNPRICED = """
-- fix: show the unknown prices on purpose
SELECT tx FROM transfers
WHERE price IS NULL;
"""

COUNT_NULLS = """
-- count the NULL prices
SELECT COUNT(*) - COUNT(price) FROM transfers;
"""

NOT_IN_PARTS = """
-- NOT IN means: not equal to each item, joined by AND
SELECT pool, pool <> 'p2', pool <> NULL, pool <> 'p2' AND pool <> NULL
FROM swaps;
"""

NOT_IN = """
-- swaps not in banned pools: one NULL empties the result
SELECT pool FROM swaps
WHERE pool NOT IN (SELECT pool FROM banned);
"""

NOT_EXISTS = """
-- fix: ask whether a matching banned row exists
SELECT pool FROM swaps s
WHERE NOT EXISTS (SELECT 1 FROM banned b WHERE b.pool = s.pool);
"""


def word(v):
    return "UNKNOWN" if v is None else ("TRUE" if v else "FALSE")


def main():
    db = sqlite3.connect(":memory:")
    db.executescript(SETUP)
    for tx, price, gt in db.execute(TRUTH):
        print(f"{tx}  price {'NULL' if price is None else price}  price > 0 = {word(gt)}")
    rows = db.execute(PRICED).fetchall()
    print(f"WHERE price > 0: {len(rows)} of 4 rows {[r[0] for r in rows]}")
    eq, ne, isnull = db.execute(NULL_COMPARE).fetchone()
    print(f"NULL = NULL {word(eq)}, NULL <> 5 {word(ne)}, NULL IS NULL {word(isnull)}")
    rows = db.execute(UNPRICED).fetchall()
    print(f"WHERE price IS NULL: {[r[0] for r in rows]}")
    print(f"COUNT(*) - COUNT(price) = {db.execute(COUNT_NULLS).fetchone()[0]}")
    print()
    for pool, a, b, both in db.execute(NOT_IN_PARTS):
        print(f"{pool}: {pool} <> p2 {word(a)}, {pool} <> NULL {word(b)}, AND = {word(both)}")
    rows = db.execute(NOT_IN).fetchall()
    print(f"NOT IN: {len(rows)} rows {[r[0] for r in rows]}")
    rows = db.execute(NOT_EXISTS).fetchall()
    print(f"NOT EXISTS: {len(rows)} rows {[r[0] for r in rows]}")


if __name__ == "__main__":
    main()
