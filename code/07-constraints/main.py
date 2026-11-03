"""Constraints and referential integrity, run in SQLite.

Part 1: no foreign key, a delete leaves three orphan lines.
Part 2: NOT NULL, CHECK, FOREIGN KEY and the three ON DELETE rules at work.
Part 3: a warehouse that does not enforce the key, and the orphan check.
SQLite enforces foreign keys only after PRAGMA foreign_keys = ON.
Run: python3 main.py
"""
import sqlite3

NO_FK = """
-- order_lines names its order, but nothing checks it
CREATE TABLE orders (order_id INT PRIMARY KEY);
CREATE TABLE order_lines (line_id TEXT, order_id INT, amount INT);
"""

WITH_RULES = """
-- every rule in the schema, checked on every write
CREATE TABLE orders (order_id INT PRIMARY KEY);
CREATE TABLE order_lines (
  line_id  TEXT,
  order_id INT NOT NULL REFERENCES orders ON DELETE RESTRICT,
  amount   INT CHECK (amount >= 0)
);
"""

WITH_CASCADE = """
-- CASCADE: deleting an order deletes its lines too
CREATE TABLE orders (order_id INT PRIMARY KEY);
CREATE TABLE order_lines (line_id TEXT, order_id INT REFERENCES orders ON DELETE CASCADE, amount INT);
"""

WITH_SET_NULL = """
-- SET NULL: the lines stay, the link is cleared
CREATE TABLE orders (order_id INT PRIMARY KEY);
CREATE TABLE order_lines (line_id TEXT, order_id INT REFERENCES orders ON DELETE SET NULL, amount INT);
"""

DATA = """
INSERT INTO orders VALUES (77), (78);
INSERT INTO order_lines VALUES ('L1', 77, 40), ('L2', 77, 25), ('L3', 77, 35), ('L4', 78, 50);
"""

DELETE_77 = "DELETE FROM orders WHERE order_id = 77;"

ORPHANS = """
-- lines whose order is gone
SELECT l.line_id FROM order_lines l
LEFT JOIN orders o ON o.order_id = l.order_id
WHERE o.order_id IS NULL;
"""

REVENUE = """
-- the revenue report sums every line
SELECT SUM(amount) FROM order_lines;
"""

BAD_INSERTS = [
    ("no order id", "INSERT INTO order_lines VALUES ('L5', NULL, 10);"),
    ("amount -5", "INSERT INTO order_lines VALUES ('L6', 78, -5);"),
    ("order 99", "INSERT INTO order_lines VALUES ('L7', 99, 10);"),
]

DELETE_BATCH = """
-- one batch of lines, its own short transaction
DELETE FROM order_lines WHERE rowid IN (
  SELECT rowid FROM order_lines WHERE order_id = ? LIMIT ?
);
"""

WAREHOUSE = """
-- the key is declared, but foreign_keys is off: nothing is checked
CREATE TABLE transactions (tx_hash TEXT PRIMARY KEY);
CREATE TABLE transfers (tx_hash TEXT REFERENCES transactions, amount INT);
"""

ORPHAN_CHECK = """
-- the data check after every load
SELECT COUNT(*) FROM transfers t
LEFT JOIN transactions x ON x.tx_hash = t.tx_hash
WHERE x.tx_hash IS NULL;
"""


def connect(schema, enforce=True):
    db = sqlite3.connect(":memory:", isolation_level=None)
    db.execute(f"PRAGMA foreign_keys = {'ON' if enforce else 'OFF'}")
    db.executescript(schema)
    db.executescript(DATA)
    return db


def try_write(db, sql):
    try:
        db.execute(sql)
        return "accepted"
    except sqlite3.IntegrityError as e:
        return f"refused: {e}"


def lines(db):
    rows = db.execute("SELECT line_id, order_id FROM order_lines ORDER BY line_id")
    return [f"{line} -> {'NULL' if order is None else order}" for line, order in rows]


def delete_in_batches(db, order_id, size):
    """Delete the children in small transactions first, then the parent."""
    statements = 0
    while True:
        n = db.execute(DELETE_BATCH, (order_id, size)).rowcount
        statements += 1
        if n == 0:
            break
    db.execute("DELETE FROM orders WHERE order_id = ?", (order_id,))
    return statements


def check_load(dw):
    """The data check: any orphan fails the load."""
    n = dw.execute(ORPHAN_CHECK).fetchone()[0]
    return n, ("failed, alert sent" if n > 0 else "ok")


def main():
    db = connect(NO_FK)
    print(f"no foreign key, delete order 77: {try_write(db, DELETE_77)}")
    print(f"orphans: {[r[0] for r in db.execute(ORPHANS)]}")
    print(f"revenue: {db.execute(REVENUE).fetchone()[0]} (order 78 alone is 50)")
    print()

    db = connect(WITH_RULES)
    for label, sql in BAD_INSERTS:
        print(f"insert line with {label}: {try_write(db, sql)}")
    print(f"RESTRICT, delete order 77: {try_write(db, DELETE_77)}")
    db = connect(WITH_CASCADE)
    print(f"CASCADE, delete order 77: {try_write(db, DELETE_77)}, lines left {lines(db)}")
    db = connect(WITH_SET_NULL)
    print(f"SET NULL, delete order 77: {try_write(db, DELETE_77)}, lines {lines(db)}")
    print()

    big = sqlite3.connect(":memory:", isolation_level=None)
    big.execute("PRAGMA foreign_keys = ON")
    big.executescript(WITH_CASCADE)
    big.execute("INSERT INTO orders VALUES (5)")
    big.executemany("INSERT INTO order_lines VALUES (?, 5, 1)", ((str(k),) for k in range(2_000_000)))
    before = big.total_changes
    big.execute("DELETE FROM orders WHERE order_id = 5")
    print(f"CASCADE on a big parent: one statement deleted {big.total_changes - before - 1:,} lines and 1 order")
    big.execute("INSERT INTO orders VALUES (6)")
    big.executemany("INSERT INTO order_lines VALUES (?, 6, 1)", ((str(k),) for k in range(2_000_000)))
    statements = delete_in_batches(big, 6, 10_000)
    print(f"fix, batches of 10,000: {statements} statements, the last finds 0, then the order")
    print()

    dw = sqlite3.connect(":memory:", isolation_level=None)
    dw.executescript(WAREHOUSE)
    dw.execute("INSERT INTO transactions VALUES ('0xab12'), ('0xcd34')")
    load = [("0xab12", 5), ("0xcd34", 7), ("0xee01", 3), ("0xee01", 4)]
    dw.executemany("INSERT INTO transfers VALUES (?, ?)", load)
    n, status = check_load(dw)
    print(f"warehouse load with 0xee01 missing: accepted, orphan check = {n}, load {status}")
    dw.execute("INSERT INTO transactions VALUES ('0xee01')")
    n, status = check_load(dw)
    print(f"after loading the transaction first: orphan check = {n}, load {status}")


if __name__ == "__main__":
    main()
