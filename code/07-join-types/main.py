"""Six join types on the same two tables, run in SQLite (3.39 or newer for RIGHT and FULL).

Order o5 names customer c9, who does not exist. Customer c5 (Eli) never ordered.
Each join answers one question: what happens to a row with no partner?
Run: python3 main.py
"""
import sqlite3

SETUP = """
CREATE TABLE orders (id TEXT, customer_id TEXT);
CREATE TABLE customers (id TEXT, name TEXT);
INSERT INTO orders VALUES ('o1', 'c1'), ('o2', 'c2'), ('o3', 'c3'), ('o4', 'c4'), ('o5', 'c9');
INSERT INTO customers VALUES ('c1', 'Ana'), ('c2', 'Ben'), ('c3', 'Cleo'), ('c4', 'Dev'), ('c5', 'Eli');
"""

INNER = """
-- INNER: only pairs survive
SELECT o.id, o.customer_id, c.name
FROM orders o
JOIN customers c
  ON c.id = o.customer_id
ORDER BY o.id;
"""

LEFT = """
-- LEFT: every order survives, NULL where no customer
SELECT o.id, o.customer_id, c.name
FROM orders o
LEFT JOIN customers c
  ON c.id = o.customer_id
ORDER BY o.id;
"""

RIGHT = """
-- RIGHT: every customer survives, NULL where no order
SELECT o.id, c.id, c.name
FROM orders o
RIGHT JOIN customers c
  ON c.id = o.customer_id
ORDER BY o.id IS NULL, o.id;
"""

FULL = """
-- FULL OUTER: both orphans survive
SELECT o.id, COALESCE(o.customer_id, c.id), c.name
FROM orders o
FULL OUTER JOIN customers c
  ON c.id = o.customer_id
ORDER BY o.id IS NULL, o.id;
"""

ANTI = """
-- ANTI: orders with no customer
SELECT o.id, o.customer_id
FROM orders o
WHERE NOT EXISTS (
  SELECT 1 FROM customers c WHERE c.id = o.customer_id
);
"""

SEMI = """
-- SEMI: orders that have a customer, once each, no customer columns
SELECT o.id, o.customer_id
FROM orders o
WHERE EXISTS (
  SELECT 1 FROM customers c WHERE c.id = o.customer_id
);
"""

LEFT_FILTER_IN_WHERE = """
-- trap: a filter on the right side in WHERE makes it an inner join
SELECT o.id, c.name FROM orders o
LEFT JOIN customers c ON c.id = o.customer_id
WHERE c.name = 'Ana';
"""

LEFT_FILTER_IN_ON = """
-- fix: the filter belongs in ON
SELECT o.id, c.name FROM orders o
LEFT JOIN customers c ON c.id = o.customer_id AND c.name = 'Ana';
"""

NOT_IN_WITH_NULL = """
-- trap: NOT IN against a list that holds one NULL id
SELECT o.id FROM orders o
WHERE o.customer_id NOT IN (SELECT id FROM customers UNION ALL SELECT NULL);
"""


def show(rows):
    return " | ".join(", ".join("NULL" if v is None else v for v in r) for r in rows)


def main():
    db = sqlite3.connect(":memory:")
    db.executescript(SETUP)
    for label, sql in [("INNER", INNER), ("LEFT", LEFT), ("RIGHT", RIGHT),
                       ("FULL OUTER", FULL), ("ANTI", ANTI), ("SEMI", SEMI)]:
        rows = db.execute(sql).fetchall()
        print(f"{label}: {len(rows)} rows  {show(rows)}")
    print()
    rows = db.execute(LEFT_FILTER_IN_WHERE).fetchall()
    print(f"LEFT, filter in WHERE: {len(rows)} row  {show(rows)}")
    rows = db.execute(LEFT_FILTER_IN_ON).fetchall()
    print(f"LEFT, filter in ON: {len(rows)} rows  {show(rows)}")
    rows = db.execute(NOT_IN_WITH_NULL).fetchall()
    print(f"NOT IN with one NULL id: {len(rows)} rows")


if __name__ == "__main__":
    main()
