"""Upsert: a write you can replay. Run in SQLite.

An indexer writes block 9506's three transfers, crashes before it saves the
checkpoint, restarts and writes block 9506 again.
Tab 1: a plain INSERT fails on the unique key, or doubles the rows without one.
Tab 2: an upsert keyed on (tx_hash, log_index) leaves the table as it was,
       and an upsert that adds to the old value counts every replay.
Run: python3 main.py
"""
import sqlite3

BLOCK_9506 = [("0xab12", 0, 5), ("0xab12", 1, 7), ("0xcd34", 0, 3)]

WITH_KEY = """
-- the key says what 'the same row' means
CREATE TABLE transfers (tx_hash TEXT, log_index INT, amount INT,
                        PRIMARY KEY (tx_hash, log_index));
CREATE TABLE checkpoint (block INT);
INSERT INTO checkpoint VALUES (9505);
"""

NO_KEY = """
-- no key: nothing can tell a replayed row from a new one
CREATE TABLE transfers (tx_hash TEXT, log_index INT, amount INT);
CREATE TABLE checkpoint (block INT);
INSERT INTO checkpoint VALUES (9505);
"""

PLAIN_INSERT = """
-- a plain insert: a second write of the same key fails
INSERT INTO transfers (tx_hash, log_index, amount) VALUES (?, ?, ?);
"""

UPSERT = """
-- a new key inserts, an existing key is updated in place
INSERT INTO transfers (tx_hash, log_index, amount) VALUES (?, ?, ?)
ON CONFLICT (tx_hash, log_index) DO UPDATE SET amount = EXCLUDED.amount;
"""

SAVE_CHECKPOINT = """
-- saved after the rows, so a crash can fall in between
UPDATE checkpoint SET block = ?;
"""

COUNTS = """
-- transfers per block, written with the block
CREATE TABLE daily_counts (block INT PRIMARY KEY, count INT);
"""

COUNT_ADDS = """
-- the trap: the new value is built from the old one
INSERT INTO daily_counts (block, count) VALUES (?, ?)
ON CONFLICT (block) DO UPDATE SET count = count + EXCLUDED.count;
"""

COUNT_SETS = """
-- the fix: set the value from the source rows
INSERT INTO daily_counts (block, count) VALUES (?, ?)
ON CONFLICT (block) DO UPDATE SET count = EXCLUDED.count;
"""


def write_block(db, write_sql, block, crash):
    for row in BLOCK_9506:
        db.execute(write_sql, row)
    if crash:
        return "crashed before the checkpoint"
    db.execute(SAVE_CHECKPOINT, (block,))
    return "checkpoint saved"


def state(db):
    rows = db.execute("SELECT COUNT(*), SUM(amount) FROM transfers").fetchone()
    cp = db.execute("SELECT block FROM checkpoint").fetchone()[0]
    return f"{rows[0]} rows, total {rows[1]}, checkpoint {cp}"


def run(schema, write_sql, label):
    db = sqlite3.connect(":memory:")
    db.executescript(schema)
    print(label)
    print("  first write:", write_block(db, write_sql, 9506, crash=True), "->", state(db))
    try:
        print("  replay:", write_block(db, write_sql, 9506, crash=False), "->", state(db))
    except sqlite3.IntegrityError as e:
        print("  replay failed:", e, "->", state(db))


def main():
    run(WITH_KEY, PLAIN_INSERT, "plain INSERT, unique key:")
    run(NO_KEY, PLAIN_INSERT, "plain INSERT, no key:")
    run(WITH_KEY, UPSERT, "upsert, unique key:")
    print()
    db = sqlite3.connect(":memory:")
    db.executescript(COUNTS)
    for _ in range(2):  # first write, then the replay
        db.execute(COUNT_ADDS, (9506, len(BLOCK_9506)))
    print("count = count + EXCLUDED.count after a replay:", db.execute("SELECT count FROM daily_counts").fetchone()[0])
    db.execute("DELETE FROM daily_counts")
    for _ in range(2):
        db.execute(COUNT_SETS, (9506, len(BLOCK_9506)))
    print("count = EXCLUDED.count after a replay:", db.execute("SELECT count FROM daily_counts").fetchone()[0])


if __name__ == "__main__":
    main()
