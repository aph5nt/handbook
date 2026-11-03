"""A materialized view kept by hand, run in SQLite.

SQLite has no materialized view, so daily_dex_volume is a real table filled
from the GROUP BY query, with a mv_state row that says how old it is.
The program builds it, refreshes one day, rewrites a matching query to read
it, then shows the two failures: stale numbers and costly full refreshes,
and a refresh that locks readers out versus one built beside the old result.
Run: python3 main.py
"""
import datetime
import os
import sqlite3
import tempfile

TODAY = "2026-10-10"
DEXES = ("aero", "raphael", "topaz", "uniswap")

SETUP = """
-- swaps: 99 days of history, 4 exchanges, 1,000 swaps each per day
CREATE TABLE swaps (day TEXT, ts TEXT, dex TEXT, amount_usd INT);
CREATE INDEX swaps_day ON swaps (day);
CREATE TABLE mv_state (name TEXT PRIMARY KEY, as_of TEXT);
"""

ADD_SWAPS = """
-- n swaps per exchange on one day, at one time
WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < :n)
INSERT INTO swaps SELECT :day, :ts, :dex, i % 97 + 1 FROM n;
"""

DAILY_VOLUME = """
-- daily volume per exchange, computed from every swap
SELECT day, dex, SUM(amount_usd) AS volume_usd, COUNT(*) AS swaps
FROM swaps GROUP BY day, dex
"""

CREATE_MV = """
-- run the query once and store the result like a table
CREATE TABLE daily_dex_volume AS
SELECT day, dex, SUM(amount_usd) AS volume_usd, COUNT(*) AS swaps
FROM swaps GROUP BY day, dex;
CREATE UNIQUE INDEX daily_dex_volume_key ON daily_dex_volume (day, dex);
"""

DASHBOARD = """
-- the dashboard reads the stored rows and the age of the copy
SELECT d.volume_usd, s.as_of FROM daily_dex_volume d, mv_state s
WHERE s.name = 'daily_dex_volume' AND d.day = :day AND d.dex = 'aero';
"""

REFRESH_DAY = """
-- incremental refresh: recompute one day's partition only
DELETE FROM daily_dex_volume WHERE day = :day;
INSERT INTO daily_dex_volume
SELECT day, dex, SUM(amount_usd), COUNT(*) FROM swaps WHERE day = :day GROUP BY day, dex;
"""

REFRESH_FULL = """
-- full refresh: throw the result away and rescan every swap
DELETE FROM daily_dex_volume;
INSERT INTO daily_dex_volume
SELECT day, dex, SUM(amount_usd), COUNT(*) FROM swaps GROUP BY day, dex;
"""

BUILD_BESIDE = """
-- build the new result beside the old one; readers still read the old
CREATE TABLE daily_dex_volume_next AS
SELECT day, dex, SUM(amount_usd) AS volume_usd, COUNT(*) AS swaps
FROM swaps GROUP BY day, dex;
"""

SWAP_IN = """
-- one short step puts the new result in place
DROP TABLE daily_dex_volume;
ALTER TABLE daily_dex_volume_next RENAME TO daily_dex_volume;
"""

BASE_TRUTH = """
-- the same number read from the base table
SELECT SUM(amount_usd) FROM swaps WHERE day = :day AND dex = 'aero';
"""

# the view's own query, so a matching query can be sent to the stored rows
VIEWS = {" ".join(DAILY_VOLUME.split()): "daily_dex_volume"}


def add_swaps(db, day, ts, n):
    for dex in DEXES:
        db.execute(ADD_SWAPS, {"n": n, "day": day, "ts": ts, "dex": dex})


def mark(db, as_of):
    db.execute("INSERT OR REPLACE INTO mv_state VALUES ('daily_dex_volume', ?)", (as_of,))


def rows_read(db, day=None):
    # the swaps column counts the base rows each stored row was built from
    sql = "SELECT SUM(swaps) FROM daily_dex_volume" + (" WHERE day = ?" if day else "")
    return db.execute(sql, (day,) if day else ()).fetchone()[0]


def run_all(db, script, params=()):
    # execute() runs one statement, so run the script's statements one by one
    for stmt in script.split(";")[:-1]:
        db.execute(stmt, params)


def refresh_day(db, day, now):
    db.execute("BEGIN")
    run_all(db, REFRESH_DAY, {"day": day})
    mark(db, now)
    db.execute("COMMIT")
    return rows_read(db, day)


def answer(db, sql):
    # query rewrite: a query that equals a view's query reads the stored rows
    target = VIEWS.get(" ".join(sql.split()))
    if target:
        return "rewritten to read " + target, db.execute("SELECT COUNT(*) FROM " + target).fetchone()[0]
    return "ran on swaps", len(db.execute(sql).fetchall())


def plan(db, sql, params=()):
    return [row[3] for row in db.execute("EXPLAIN QUERY PLAN " + sql, params)]


def main():
    path = os.path.join(tempfile.mkdtemp(), "swaps.db")
    db = sqlite3.connect(path, isolation_level=None)
    db.executescript(SETUP)
    first = datetime.date.fromisoformat(TODAY) - datetime.timedelta(days=99)
    db.execute("BEGIN")
    for k in range(99):
        add_swaps(db, str(first + datetime.timedelta(days=k)), "12:00", 1000)
    add_swaps(db, TODAY, "08:00", 1000)
    db.execute("COMMIT")

    print("== refresh and read")
    print("swaps:", db.execute("SELECT COUNT(*) FROM swaps").fetchone()[0])
    print("page load without a view, plan:", plan(db, DAILY_VOLUME))
    db.executescript(CREATE_MV)
    mark(db, "08:30")
    print("daily_dex_volume rows:", db.execute("SELECT COUNT(*) FROM daily_dex_volume").fetchone()[0])
    print("dashboard plan:", plan(db, DASHBOARD, {"day": TODAY}))
    print("dashboard reads:", db.execute(DASHBOARD, {"day": TODAY}).fetchone())
    add_swaps(db, TODAY, "08:45", 250)
    print("refresh today at 09:00 reads", refresh_day(db, TODAY, "09:00"), "swaps")
    print("analyst query:", answer(db, DAILY_VOLUME))

    print("== stale and costly")
    add_swaps(db, TODAY, "10:00", 250)
    print("10:00 dashboard (volume, as of):", db.execute(DASHBOARD, {"day": TODAY}).fetchone(),
          " base table:", db.execute(BASE_TRUTH, {"day": TODAY}).fetchone()[0])
    db.executescript(REFRESH_FULL)
    full = rows_read(db)
    print("one full refresh reads", full, "swaps; every 5 minutes is", full * 12, "an hour")

    reader = sqlite3.connect(path, timeout=0)
    db.execute("BEGIN EXCLUSIVE")
    run_all(db, REFRESH_FULL)
    try:
        reader.execute("SELECT COUNT(*) FROM daily_dex_volume").fetchone()
        print("reader during a locking refresh: read")
    except sqlite3.OperationalError as err:
        print("reader during a locking refresh:", err)
    db.execute("COMMIT")

    print("refresh today at 10:05 reads", refresh_day(db, TODAY, "10:05"), "swaps")
    print("10:05 dashboard (volume, as of):", db.execute(DASHBOARD, {"day": TODAY}).fetchone(),
          " base table:", db.execute(BASE_TRUTH, {"day": TODAY}).fetchone()[0])

    db.execute("BEGIN")
    db.execute(BUILD_BESIDE)
    print("reader while the new result is built:",
          reader.execute("SELECT COUNT(*) FROM daily_dex_volume").fetchone()[0], "rows")
    run_all(db, SWAP_IN)
    db.execute("COMMIT")
    print("reader after the swap:", reader.execute("SELECT COUNT(*) FROM daily_dex_volume").fetchone()[0], "rows")


main()
