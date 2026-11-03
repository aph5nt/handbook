"""Normalise or copy, run in SQLite.

Part 1 loads one wide swap table where pool p1 has 1,000,000 swaps and a
typo in token1, checks 1NF, 2NF and 3NF on the rows, splits the table,
and counts the rows each shape must update to fix the typo.
Part 2 copies a block timestamp onto transfers, corrects the source,
shows the copy drift, rebuilds it, and counts the tables a normalised
daily report must read.
Run: python3 main.py
"""
import sqlite3

WIDE = """
-- one wide table: every swap repeats its pool's tokens and exchange
CREATE TABLE pool_swaps (pool TEXT, tx_hash TEXT, amount INT, token0 TEXT,
  token1 TEXT, dex_name TEXT, dex_version TEXT, PRIMARY KEY (pool, tx_hash));
WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 1000000)
INSERT INTO pool_swaps
SELECT 'p1', printf('0xa%d', i), i % 200, 'WETH', 'USCD', 'aerodrome', 'v2' FROM n;
"""

CHECK_1NF = """
-- 1NF: a cell holding a list would carry a comma
SELECT COUNT(*) FROM pool_swaps
WHERE token0 LIKE '%,%' OR token1 LIKE '%,%' OR dex_name LIKE '%,%';
"""

CHECK_2NF = """
-- 2NF: does each pool column take one value per pool? Then it needs pool alone
SELECT pool, COUNT(DISTINCT token0), COUNT(DISTINCT token1),
       COUNT(DISTINCT dex_name), COUNT(DISTINCT dex_version)
FROM pool_swaps GROUP BY pool;
"""

SPLIT_2NF = """
-- move the columns that need only pool into pools; swaps keeps its key
CREATE TABLE pools (pool TEXT PRIMARY KEY, token0 TEXT, token1 TEXT,
  dex_name TEXT, dex_version TEXT);
INSERT INTO pools SELECT DISTINCT pool, token0, token1, dex_name, dex_version
FROM pool_swaps;
CREATE TABLE swaps (pool TEXT REFERENCES pools (pool), tx_hash TEXT, amount INT,
  PRIMARY KEY (pool, tx_hash));
INSERT INTO swaps SELECT pool, tx_hash, amount FROM pool_swaps;
"""

CHECK_3NF = """
-- 3NF: does dex_version take one value per dex_name? Then dex_name decides it
SELECT dex_name, COUNT(DISTINCT dex_version) FROM pools GROUP BY dex_name;
"""

SPLIT_3NF = """
-- move dex_version to dexes, keyed by dex_name
CREATE TABLE dexes (dex_name TEXT PRIMARY KEY, dex_version TEXT);
INSERT INTO dexes SELECT DISTINCT dex_name, dex_version FROM pools;
ALTER TABLE pools DROP COLUMN dex_version;
"""

FIX_NORMALISED = """
-- the typo lives in one pools row
UPDATE pools SET token1 = 'USDC' WHERE pool = 'p1' AND token1 = 'USCD';
"""

FIX_WIDE = """
-- the typo lives in every swap row of p1
UPDATE pool_swaps SET token1 = 'USDC' WHERE pool = 'p1' AND token1 = 'USCD';
"""

READ_THROUGH_JOIN = """
-- every swap of p1 reads the fixed name through the join
SELECT p.token1, COUNT(*) FROM swaps s JOIN pools p ON p.pool = s.pool
GROUP BY p.token1;
"""

TRANSFERS = """
-- normalised: a transfer knows its block height, blocks know the time
CREATE TABLE blocks (height INT PRIMARY KEY, timestamp TEXT);
CREATE TABLE transfers (tx_hash TEXT PRIMARY KEY, block_height INT, amount INT,
  token TEXT);
INSERT INTO blocks VALUES (9506, '2026-10-01 10:00:00'), (9507, '2026-10-01 10:00:01');
INSERT INTO transfers VALUES ('0xa1', 9506, 120, 'T1'), ('0xa2', 9506, 80, 'T1'),
  ('0xa3', 9507, 45, 'T1');
"""

DAY_BY_JOIN = """
-- the day report must join every transfer to blocks
SELECT COUNT(*) FROM transfers t JOIN blocks b ON b.height = t.block_height
WHERE date(b.timestamp) = ?;
"""

BUILD_COPY = """
-- denormalise on purpose: copy block_ts onto every transfer
DROP TABLE IF EXISTS transfers_wide;
CREATE TABLE transfers_wide AS
SELECT t.tx_hash, t.block_height, t.amount, b.timestamp AS block_ts
FROM transfers t JOIN blocks b ON b.height = t.block_height;
"""

DAY_FROM_COPY = """
-- the day report reads one table, no join
SELECT COUNT(*) FROM transfers_wide WHERE date(block_ts) = ?;
"""

CORRECT_SOURCE = """
-- block 9506's time came from a bad node; the source is corrected
UPDATE blocks SET timestamp = '2026-09-30 23:59:59' WHERE height = 9506;
"""

DIM_TABLES = """
-- the warehouse built like an app database: tokens, pools and dexes apart
CREATE TABLE tokens (token TEXT PRIMARY KEY, pool TEXT);
CREATE TABLE pools_n (pool TEXT PRIMARY KEY, dex_name TEXT);
CREATE TABLE dexes_n (dex_name TEXT PRIMARY KEY, dex_version TEXT);
INSERT INTO tokens VALUES ('T1', 'p1');
INSERT INTO pools_n VALUES ('p1', 'aerodrome');
INSERT INTO dexes_n VALUES ('aerodrome', 'v2');
"""

DAILY_VOLUME_NORMALISED = """
-- daily volume per exchange: transfers joined to four more tables
SELECT date(b.timestamp) AS day, d.dex_name, d.dex_version, SUM(t.amount)
FROM transfers t
JOIN blocks b ON b.height = t.block_height
JOIN tokens k ON k.token = t.token
JOIN pools_n p ON p.pool = k.pool
JOIN dexes_n d ON d.dex_name = p.dex_name
GROUP BY day, d.dex_name, d.dex_version;
"""


def one(db, sql, *args):
    return db.execute(sql, args).fetchone()[0]


def tables_read(db, sql):
    # one plan row per table the engine scans or searches
    plan = db.execute("EXPLAIN QUERY PLAN " + sql).fetchall()
    return sum(1 for row in plan if row[3].startswith(("SCAN", "SEARCH")))


def main():
    db = sqlite3.connect(":memory:")

    db.executescript(WIDE)
    print("pool_swaps rows of p1:", one(db, "SELECT COUNT(*) FROM pool_swaps WHERE pool = 'p1'"))
    print("rows holding the typo USCD:", one(db, "SELECT COUNT(*) FROM pool_swaps WHERE token1 = 'USCD'"))
    print("1NF cells holding a list:", one(db, CHECK_1NF))
    print("2NF distinct values per pool (token0, token1, dex_name, dex_version):",
          db.execute(CHECK_2NF).fetchall())
    db.executescript(SPLIT_2NF)
    print("after 2NF split: swaps", one(db, "SELECT COUNT(*) FROM swaps"),
          "rows, pools", one(db, "SELECT COUNT(*) FROM pools"), "row")
    print("3NF distinct dex_version per dex_name:", db.execute(CHECK_3NF).fetchall())
    db.executescript(SPLIT_3NF)
    print("after 3NF split: dexes", db.execute("SELECT * FROM dexes").fetchall())
    print("fix the typo, normalised: rows updated =", db.execute(FIX_NORMALISED).rowcount)
    print("swaps reading token1 through the join:", db.execute(READ_THROUGH_JOIN).fetchall())
    print("fix the typo, wide table: rows updated =", db.execute(FIX_WIDE).rowcount)

    db.executescript(TRANSFERS)
    print("2026-10-01 by join:", one(db, DAY_BY_JOIN, "2026-10-01"))
    db.executescript(BUILD_COPY)
    print("2026-10-01 from copy:", one(db, DAY_FROM_COPY, "2026-10-01"))
    db.executescript(CORRECT_SOURCE)
    print("after the source fix, copy says 2026-10-01:", one(db, DAY_FROM_COPY, "2026-10-01"),
          "and 2026-09-30:", one(db, DAY_FROM_COPY, "2026-09-30"))
    print("after the source fix, join says 2026-10-01:", one(db, DAY_BY_JOIN, "2026-10-01"),
          "and 2026-09-30:", one(db, DAY_BY_JOIN, "2026-09-30"))
    db.executescript(BUILD_COPY)
    print("rebuilt copy says 2026-10-01:", one(db, DAY_FROM_COPY, "2026-10-01"),
          "and 2026-09-30:", one(db, DAY_FROM_COPY, "2026-09-30"))

    db.executescript(DIM_TABLES)
    print("daily volume, normalised:", db.execute(DAILY_VOLUME_NORMALISED).fetchall())
    print("tables the normalised report reads:", tables_read(db, DAILY_VOLUME_NORMALISED))
    print("tables the copied report reads:", tables_read(db, DAY_FROM_COPY.replace("?", "'2026-10-01'")))


main()
