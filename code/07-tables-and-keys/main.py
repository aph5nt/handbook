"""Tables and keys: which row is which thing. Run in SQLite.

Tab 1: an indexer writes block 9506 into blocks, transactions and transfers.
       Keys are values, a join follows them, and rows have no order without ORDER BY.
Tab 2: users keyed by e-mail break when the e-mail changes; a surrogate key fixes it.
Run: python3 main.py
"""
import hashlib
import sqlite3
import struct

CHAIN_SCHEMA = """
-- one table per kind of thing, a row holds key values, never pointers
CREATE TABLE blocks (
  height INT PRIMARY KEY,                       -- natural key: the chain made it
  hash   TEXT
);
CREATE TABLE transactions (
  tx_hash      TEXT PRIMARY KEY,                -- natural key
  block_height INT REFERENCES blocks (height)   -- foreign key: a copy of the block's key
);
CREATE TABLE transfers (
  transfer_id INTEGER PRIMARY KEY,              -- surrogate key: the indexer made it up
  tx_hash     TEXT REFERENCES transactions (tx_hash)
);
CREATE INDEX transfers_by_tx ON transfers (tx_hash DESC, transfer_id);
"""

WRITE_BLOCK = "INSERT INTO blocks VALUES (?, ?)"
WRITE_TX = "INSERT INTO transactions VALUES (?, ?)"
WRITE_TRANSFER = "INSERT INTO transfers (tx_hash) VALUES (?)"

FOLLOW_KEYS = """
-- a join walks the same links: transfer -> transaction -> block
SELECT f.transfer_id, f.tx_hash, b.height
FROM transfers f
JOIN transactions t ON t.tx_hash = f.tx_hash
JOIN blocks b ON b.height = t.block_height
WHERE f.transfer_id <= 3
ORDER BY f.transfer_id;
"""

NO_ORDER = """
-- no ORDER BY: the engine returns rows in whatever order its plan reads them
SELECT transfer_id, tx_hash FROM transfers
WHERE tx_hash IN ('0xab12', '0xcd34');
"""

EMAIL_KEYED = """
-- the app keys users by e-mail, each order copies it
CREATE TABLE users  (email TEXT PRIMARY KEY, name TEXT);
CREATE TABLE orders (order_id INT PRIMARY KEY, user_email TEXT);
INSERT INTO users  VALUES ('ana@old.com', 'Ana');
INSERT INTO orders VALUES (501, 'ana@old.com'), (502, 'ana@old.com'), (503, 'ana@old.com');
"""

CHANGE_EMAIL_KEY = "UPDATE users SET email = 'ana@new.com' WHERE email = 'ana@old.com'"
ORDERS_OF_ANA_BY_EMAIL = """
SELECT o.order_id FROM orders o JOIN users u ON u.email = o.user_email
WHERE u.name = 'Ana';
"""

SURROGATE_KEYED = """
-- the fix: a made-up id is the key, the e-mail is a plain column
CREATE TABLE users  (user_id INT PRIMARY KEY, email TEXT, name TEXT);
CREATE TABLE orders (order_id INT PRIMARY KEY, user_id INT);
INSERT INTO users  VALUES (42, 'ana@old.com', 'Ana');
INSERT INTO orders VALUES (501, 42), (502, 42), (503, 42);
"""

CHANGE_EMAIL_FACT = "UPDATE users SET email = 'ana@new.com' WHERE user_id = 42"
ORDERS_OF_ANA_BY_ID = """
SELECT o.order_id FROM orders o JOIN users u ON u.user_id = o.user_id
WHERE u.name = 'Ana';
"""


def index_block(db):
    db.execute(WRITE_BLOCK, (9506, "0x7f3c"))
    tx_hashes = ["0xab12", "0xcd34"] + [f"0x{n:04x}" for n in range(1, 39)]
    for h in tx_hashes:
        db.execute(WRITE_TX, (h, 9506))
    emitters = ["0xab12", "0xab12", "0xcd34"] + [tx_hashes[2 + n % 38] for n in range(87)]
    for h in emitters:
        db.execute(WRITE_TRANSFER, (h,))


def main():
    db = sqlite3.connect(":memory:")
    db.execute("PRAGMA foreign_keys = ON")
    db.executescript(CHAIN_SCHEMA)
    index_block(db)
    for t in ("blocks", "transactions", "transfers"):
        print(f"{t}: {db.execute(f'SELECT COUNT(*) FROM {t}').fetchone()[0]} rows")
    try:
        db.execute(WRITE_BLOCK, (9506, "0x7f3c"))
    except sqlite3.IntegrityError as e:
        print("second block 9506:", e)
    try:
        db.execute(WRITE_TX, ("0xee99", 9999))
    except sqlite3.IntegrityError as e:
        print("transaction in block 9999, which no row holds:", e)
    print("follow the keys:", db.execute(FOLLOW_KEYS).fetchall())
    print("no ORDER BY:", [r[0] for r in db.execute(NO_ORDER)])
    print("  plan:", [r[3] for r in db.execute("EXPLAIN QUERY PLAN " + NO_ORDER)])
    print()

    app = sqlite3.connect(":memory:")
    app.executescript(EMAIL_KEYED)
    print("orders of Ana, e-mail key:", [r[0] for r in app.execute(ORDERS_OF_ANA_BY_EMAIL)])
    app.execute(CHANGE_EMAIL_KEY)
    print("after the e-mail change:", [r[0] for r in app.execute(ORDERS_OF_ANA_BY_EMAIL)],
          "orders still say", app.execute("SELECT DISTINCT user_email FROM orders").fetchone()[0])

    app = sqlite3.connect(":memory:")
    app.executescript(SURROGATE_KEYED)
    changed = app.execute(CHANGE_EMAIL_FACT).rowcount
    print("surrogate key: rows changed", changed, "-> orders of Ana:", [r[0] for r in app.execute(ORDERS_OF_ANA_BY_ID)])
    print()

    tx_hash = hashlib.sha256(b"0xab12").digest()
    print("join key width: tx_hash", len(tx_hash), "bytes, tx_id BIGINT", struct.calcsize("q"), "bytes")


if __name__ == "__main__":
    main()
