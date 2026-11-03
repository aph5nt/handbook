"""Set operations on two address lists, run in SQLite.

List A received money from a mixer. List B sent money to an exchange.
The second part shows UNION dropping one of two real, equal transfers.
Run: python3 main.py
"""
import sqlite3

SETUP = """
CREATE TABLE list_a (addr TEXT);
CREATE TABLE list_b (addr TEXT);
INSERT INTO list_a VALUES ('x'), ('y'), ('z');
INSERT INTO list_b VALUES ('y'), ('z'), ('w');
CREATE TABLE part1 (tx_hash TEXT, log_index INT, sender TEXT, receiver TEXT, amount INT);
CREATE TABLE part2 (tx_hash TEXT, log_index INT, sender TEXT, receiver TEXT, amount INT);
INSERT INTO part1 VALUES ('0xab12', 0, 'a', 'b', 5), ('0xab12', 1, 'a', 'b', 5);
INSERT INTO part2 VALUES ('0xcd34', 0, 'c', 'd', 7);
"""

UNION_ALL = """
-- stack both lists, keep repeats
SELECT addr FROM list_a
UNION ALL
SELECT addr FROM list_b;
"""

UNION = """
-- stack, then keep one copy of each row
SELECT addr FROM list_a
UNION
SELECT addr FROM list_b
ORDER BY addr;
"""

INTERSECT = """
-- rows found in both lists
SELECT addr FROM list_a
INTERSECT
SELECT addr FROM list_b
ORDER BY addr;
"""

A_EXCEPT_B = """
-- rows of A that B lacks
SELECT addr FROM list_a
EXCEPT
SELECT addr FROM list_b;
"""

B_EXCEPT_A = """
-- order matters: rows of B that A lacks
SELECT addr FROM list_b
EXCEPT
SELECT addr FROM list_a;
"""

TRAP_UNION_ALL = """
-- every real transfer, repeats kept
SELECT sender, receiver, amount FROM part1
UNION ALL
SELECT sender, receiver, amount FROM part2;
"""

TRAP_UNION = """
-- UNION by mistake: two equal payments look like one
SELECT sender, receiver, amount FROM part1
UNION
SELECT sender, receiver, amount FROM part2;
"""

FIX_WITH_KEY = """
-- the real key keeps the two payments apart
SELECT tx_hash, log_index, sender, receiver, amount FROM part1
UNION
SELECT tx_hash, log_index, sender, receiver, amount FROM part2;
"""


def run(db, label, sql):
    rows = db.execute(sql).fetchall()
    print(f"{label}: {len(rows)} rows {[r[0] if len(r) == 1 else r for r in rows]}")
    return rows


def total(rows):
    return sum(r[-1] for r in rows)


def main():
    db = sqlite3.connect(":memory:")
    db.executescript(SETUP)
    run(db, "A UNION ALL B", UNION_ALL)
    run(db, "A UNION B", UNION)
    run(db, "A INTERSECT B", INTERSECT)
    run(db, "A EXCEPT B", A_EXCEPT_B)
    run(db, "B EXCEPT A", B_EXCEPT_A)
    print()
    rows = run(db, "transfers UNION ALL", TRAP_UNION_ALL)
    print(f"  total amount: {total(rows)}")
    rows = run(db, "transfers UNION", TRAP_UNION)
    print(f"  total amount: {total(rows)}  (one payment of 5 lost)")
    rows = run(db, "transfers UNION with key", FIX_WITH_KEY)
    print(f"  total amount: {total(rows)}")


if __name__ == "__main__":
    main()
