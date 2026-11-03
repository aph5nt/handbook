"""A view as the read-only door, run in SQLite.

SQLite has views but no users and no GRANT. A grants table names what the
partner may read, and an authorizer callback enforces it on every column
the engine reads: a read is allowed when the table itself is granted, or
when it happens inside a granted view.
Part 1 is the door, part 2 a stored copy beside it, part 3 stacked views.
Run: python3 main.py
"""
import sqlite3

SETUP = """
-- five public columns, two internal ones, rows for two chains
CREATE TABLE transfers (chain TEXT, tx_hash TEXT, from_address TEXT,
  to_address TEXT, amount INT, block_ts TEXT, ingest_node TEXT, risk_note TEXT);
CREATE INDEX transfers_day ON transfers (block_ts);
INSERT INTO transfers VALUES
  ('robinhood', '0xa1', '0xf1', '0xt1', 120, '2026-10-01', 'node-3', 'watch list'),
  ('robinhood', '0xa2', '0xf2', '0xt2', 80, '2026-10-02', 'node-3', ''),
  ('base', '0xb1', '0xf3', '0xt3', 45, '2026-10-02', 'node-7', 'under review');
CREATE TABLE grants (grantee TEXT, object TEXT);
"""

GRANT_BASE = """
-- the quick way: the partner may read the base table
INSERT INTO grants VALUES ('partner', 'transfers');
"""

CREATE_VIEW = """
-- a saved query: five public columns, one chain
CREATE VIEW transfers_public AS
SELECT tx_hash, from_address, to_address, amount, block_ts
FROM transfers WHERE chain = 'robinhood';
"""

GRANT_VIEW = """
-- the view is the only door: no grant on transfers
DELETE FROM grants WHERE grantee = 'partner';
INSERT INTO grants VALUES ('partner', 'transfers_public');
"""

NEW_ROW = """
-- the indexer writes a new robinhood transfer
INSERT INTO transfers VALUES
  ('robinhood', '0xa3', '0xf4', '0xt4', 60, '2026-10-02', 'node-3', '');
"""

CHANGE_BASE = """
-- the team renames an internal column and adds one
ALTER TABLE transfers RENAME COLUMN ingest_node TO source_node;
ALTER TABLE transfers ADD COLUMN gas_used INT;
"""

BUILD_MV = """
-- SQLite has no materialized view: a table filled from the query is the same thing
DROP TABLE IF EXISTS daily_volume_mv;
CREATE TABLE daily_volume_mv AS
SELECT block_ts AS day, SUM(amount) AS volume
FROM transfers WHERE chain = 'robinhood' GROUP BY block_ts;
"""

DAY_FROM_VIEW = """
-- the view runs its query on this read
SELECT SUM(amount) FROM transfers_public WHERE block_ts = '2026-10-02';
"""

DAY_FROM_MV = """
-- the stored copy answers from its own rows
SELECT volume FROM daily_volume_mv WHERE day = '2026-10-02';
"""

COPY_ALL_COLUMNS = """
-- a copy built from every column, then granted for speed
CREATE TABLE transfers_mv AS SELECT * FROM transfers WHERE chain = 'robinhood';
INSERT INTO grants VALUES ('partner', 'transfers_mv');
"""

STACK = """
-- one view per team, each reading the one below
CREATE VIEW view_a AS SELECT DISTINCT chain, tx_hash, amount, block_ts AS day FROM transfers;
CREATE VIEW view_b AS SELECT chain, day, SUM(amount) AS volume FROM view_a GROUP BY chain, day;
CREATE VIEW view_c AS SELECT day, volume FROM view_b WHERE chain = 'robinhood';
"""

TODAY_STACKED = """
-- today's volume through three layers
SELECT volume FROM view_c WHERE day = '2026-10-02';
"""

CHANGE_VIEW_A = """
-- view_a's team drops a column they no longer need
DROP VIEW view_a;
CREATE VIEW view_a AS SELECT DISTINCT chain, tx_hash, block_ts AS day FROM transfers;
"""

PROBE = """
-- the reader's own function in the WHERE clause records every row it is shown
SELECT tx_hash FROM transfers_public WHERE peek(tx_hash);
"""

FLAT = """
-- one layer, base tables only, filter and columns written out
CREATE VIEW daily_volume AS
SELECT block_ts AS day, SUM(amount) AS volume
FROM transfers WHERE chain = 'robinhood' GROUP BY block_ts;
"""

TODAY_FLAT = """
-- today's volume through one layer
SELECT volume FROM daily_volume WHERE day = '2026-10-02';
"""

def as_partner(db):
    # read the grants once, then check every column the engine reads
    granted = {o for (o,) in db.execute("SELECT object FROM grants WHERE grantee = 'partner'")}

    def check(action, table, column, schema, inside_view):
        if action != sqlite3.SQLITE_READ:
            return sqlite3.SQLITE_OK
        if table in granted or inside_view in granted:
            return sqlite3.SQLITE_OK
        return sqlite3.SQLITE_DENY

    db.set_authorizer(check)


def as_owner(db):
    db.set_authorizer(None)


def read(db, sql):
    try:
        cur = db.execute(sql)
        return [d[0] for d in cur.description], cur.fetchall()
    except sqlite3.DatabaseError as err:
        return "error", str(err)


def plan(db, sql):
    return [row[3] for row in db.execute("EXPLAIN QUERY PLAN " + sql)]


def fresh():
    db = sqlite3.connect(":memory:")
    db.executescript(SETUP)
    return db


def door():
    db = fresh()
    print("columns:", [r[1] for r in db.execute("PRAGMA table_info(transfers)")])
    db.executescript(GRANT_BASE)
    as_partner(db)
    cols, rows = read(db, "SELECT chain, tx_hash, risk_note FROM transfers")
    print("grant on the base table, partner reads:", rows)
    as_owner(db)
    db.executescript(CREATE_VIEW)
    db.executescript(GRANT_VIEW)
    as_partner(db)
    cols, rows = read(db, "SELECT * FROM transfers_public")
    print("grant on the view, partner sees", len(cols), "columns:", cols)
    print("  rows:", [r[0] for r in rows])
    print("partner reads transfers directly:", read(db, "SELECT risk_note FROM transfers"))
    as_owner(db)
    db.executescript(NEW_ROW)
    as_partner(db)
    print("after the new row, partner sees:", [r[0] for r in read(db, "SELECT * FROM transfers_public")[1]])
    as_owner(db)
    db.executescript(CHANGE_BASE)
    as_partner(db)
    cols, rows = read(db, "SELECT * FROM transfers_public")
    print("after the base change, the view still has", len(cols), "columns, rows:", len(rows))


def mv():
    db = fresh()
    db.executescript(CREATE_VIEW)
    print("plan of a view read:", plan(db, DAY_FROM_VIEW))
    db.executescript(BUILD_MV)
    print("plan of a stored-copy read:", plan(db, DAY_FROM_MV))
    db.executescript(NEW_ROW)
    print("2026-10-02 volume, view:", db.execute(DAY_FROM_VIEW).fetchone()[0],
          " stored copy:", db.execute(DAY_FROM_MV).fetchone()[0])
    db.executescript(BUILD_MV)
    print("after refresh, stored copy:", db.execute(DAY_FROM_MV).fetchone()[0])
    db.executescript(COPY_ALL_COLUMNS)
    as_partner(db)
    print("partner reads transfers:", read(db, "SELECT risk_note FROM transfers")[0])
    print("partner reads the granted copy:", read(db, "SELECT tx_hash, risk_note FROM transfers_mv")[1])


def stack():
    db = fresh()
    db.executescript(STACK)
    print("stacked, today:", db.execute(TODAY_STACKED).fetchall())
    print("stacked plan:", plan(db, TODAY_STACKED))
    db.executescript(CHANGE_VIEW_A)
    print("view_c after view_a changed:", read(db, TODAY_STACKED))
    db.executescript(CREATE_VIEW)
    seen = []
    db.create_function("peek", 1, lambda tx: seen.append(tx) or 1)
    db.execute(PROBE).fetchall()
    print("rows the reader's function was shown in SQLite:", seen)
    db.executescript(FLAT)
    print("flat, today:", db.execute(TODAY_FLAT).fetchall())
    print("flat plan:", plan(db, TODAY_FLAT))


print("== door"); door()
print("== view or stored copy"); mv()
print("== stacked views"); stack()
