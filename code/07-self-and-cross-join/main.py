"""Self join and cross join, run in SQLite.

Part 1: transfers joined to itself finds money that moved on within one hour.
Part 2: days CROSS JOIN pools builds a grid, so days with no swaps show 0.
Each part ends with the rows a join multiplies by mistake.
Run: python3 main.py
"""
import sqlite3

# ts is minutes after midnight: 600 is 10:00.
SETUP = """
CREATE TABLE transfers (from_address TEXT, to_address TEXT, amount INT, ts INT);
INSERT INTO transfers VALUES ('S', 'M', 50, 600), ('M', 'X', 48, 620), ('M', 'Y', 2, 780), ('K', 'S', 30, 480);
CREATE TABLE days (day TEXT);
CREATE TABLE pools (pool TEXT);
CREATE TABLE daily_swaps (day TEXT, pool TEXT, n INT);
INSERT INTO days VALUES ('1 Oct'), ('2 Oct');
INSERT INTO pools VALUES ('p1'), ('p2');
INSERT INTO daily_swaps VALUES ('1 Oct', 'p1', 12), ('2 Oct', 'p2', 5);
"""

HOPS = """
-- name the table twice and link the hops
SELECT a.from_address, a.to_address, b.to_address, b.ts - a.ts, a.ts
FROM transfers a JOIN transfers b
  ON b.from_address = a.to_address
ORDER BY a.ts;
"""

FAST_HOPS = """
-- money that moved on within one hour
SELECT a.from_address, a.to_address AS middle, b.to_address, a.amount, b.amount
FROM transfers a JOIN transfers b
  ON b.from_address = a.to_address
 AND b.ts BETWEEN a.ts AND a.ts + 60;
"""

GROUP_ONLY = """
-- GROUP BY sees only the rows that exist
SELECT day, pool, SUM(n) FROM daily_swaps
GROUP BY day, pool;
"""

GRID = """
-- every day with every pool
SELECT d.day, p.pool
FROM days d CROSS JOIN pools p
ORDER BY d.day, p.pool;
"""

GRID_COUNTS = """
-- left join the counts onto the grid
SELECT d.day, p.pool, s.n
FROM days d CROSS JOIN pools p
LEFT JOIN daily_swaps s ON s.day = d.day AND s.pool = p.pool
ORDER BY d.day, p.pool;
"""

DASHBOARD = """
-- no row means no swaps, so 0 is the true answer
SELECT d.day, p.pool, COALESCE(s.n, 0) AS swaps
FROM days d CROSS JOIN pools p
LEFT JOIN daily_swaps s ON s.day = d.day AND s.pool = p.pool
ORDER BY d.day, p.pool;
"""

BUSY_SETUP = """
CREATE TABLE busy (from_address TEXT, to_address TEXT, ts INT);
CREATE TABLE many_pools (pool INT);
"""

FAN_OUT = """
-- a busy address, no time bound: every in pairs with every out
SELECT COUNT(*)
FROM busy a JOIN busy b
  ON b.from_address = a.to_address;
"""

COMMA_JOIN = """
-- the comma form with the WHERE forgotten: every row with every row
SELECT COUNT(*)
FROM busy, many_pools;
"""


def hhmm(minutes):
    return f"{minutes // 60:02d}:{minutes % 60:02d}"


def main():
    db = sqlite3.connect(":memory:")
    db.executescript(SETUP)
    for a, m, b, gap, t in db.execute(HOPS):
        print(f"candidate: {a} to {m} at {hhmm(t)}, then {m} to {b}, {gap} minutes later")
    for a, m, b, x, y in db.execute(FAST_HOPS):
        print(f"within one hour: {a} sent {x} to {m}, {m} sent {y} to {b}")
    rows = db.execute(FAST_HOPS).fetchall()
    print(f"self join with time bound: {len(rows)} row")
    print()
    rows = db.execute(GROUP_ONLY).fetchall()
    print(f"GROUP BY alone: {len(rows)} rows {rows}")
    rows = db.execute(GRID).fetchall()
    print(f"days CROSS JOIN pools: {len(rows)} rows {rows}")
    print(f"grid LEFT JOIN counts: {['NULL' if r[2] is None else r[2] for r in db.execute(GRID_COUNTS)]}")
    print(f"dashboard: {[r[2] for r in db.execute(DASHBOARD)]}")
    print()
    db.executescript(BUSY_SETUP)
    ins = [("in" + str(k), "M", k) for k in range(100)] + [("M", "out" + str(k), k) for k in range(100)]
    db.executemany("INSERT INTO busy VALUES (?, ?, ?)", ins)
    db.executemany("INSERT INTO many_pools VALUES (?)", [(k,) for k in range(1000)])
    print(f"busy M, 100 in and 100 out, no time bound: {db.execute(FAN_OUT).fetchone()[0]} pairs")
    print(f"at 10,000 in and 10,000 out: {10_000 * 10_000:,} pairs")
    print(f"comma join, 200 rows x 1,000 pools: {db.execute(COMMA_JOIN).fetchone()[0]:,} rows")
    print(f"at 1,000,000 x 1,000,000: {1_000_000 * 1_000_000:,} rows")


if __name__ == "__main__":
    main()
