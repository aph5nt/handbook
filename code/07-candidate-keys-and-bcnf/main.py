"""Candidate keys and BCNF on trader_routes, run in SQLite.

The rules come from the business: a trader uses one router per exchange,
and each router belongs to one exchange. The program finds the candidate
keys from those rules, tests BCNF and 3NF, shows the lost fact, then
splits the table and shows the rule the split can no longer hold.
Run: python3 main.py
"""
import itertools
import sqlite3

COLS = ("trader", "dex", "router")
# functional dependencies: left side decides right side
FDS = [(("trader", "dex"), "router"), (("router",), "dex")]

SETUP = """
-- three rows: ana uses R1 on Aero and R7 on Topaz, ben uses R1 on Aero
CREATE TABLE trader_routes (trader TEXT, dex TEXT, router TEXT);
INSERT INTO trader_routes VALUES ('ana', 'Aero', 'R1');
INSERT INTO trader_routes VALUES ('ben', 'Aero', 'R1');
INSERT INTO trader_routes VALUES ('ana', 'Topaz', 'R7');
"""

REPEATS = """
-- value combinations of these columns that appear more than once
SELECT {cols}, COUNT(*) AS n FROM trader_routes
GROUP BY {cols}
HAVING COUNT(*) > 1;
"""

BREAKS_FD = """
-- rows where one left value maps to two right values (empty: the rule holds)
SELECT {left} FROM trader_routes
GROUP BY {left}
HAVING COUNT(DISTINCT {right}) > 1;
"""

LOSE_FACT = """
-- delete ana's Topaz row, then ask which exchange R7 belongs to
DELETE FROM trader_routes WHERE trader = 'ana' AND router = 'R7';
"""

ROUTER_DEX = """
-- where does R7 belong?
SELECT router, dex FROM trader_routes WHERE router = 'R7';
"""

SPLIT = """
-- each router once, keyed by router; who uses what, keyed by both
CREATE TABLE routers (router TEXT PRIMARY KEY, dex TEXT NOT NULL);
CREATE TABLE trader_routers (
  trader TEXT, router TEXT REFERENCES routers (router),
  PRIMARY KEY (trader, router));
INSERT INTO routers SELECT DISTINCT router, dex FROM trader_routes;
INSERT INTO trader_routers SELECT trader, router FROM trader_routes;
"""

DELETE_USE = """
-- remove only the fact 'ana uses R7'
DELETE FROM trader_routers WHERE trader = 'ana' AND router = 'R7';
"""

STILL_THERE = """
-- routers still holds the fact
SELECT router, dex FROM routers WHERE router = 'R7';
"""

ADD_SECOND_ROUTER = """
-- R2 is also on Aero; ana adds it, and both tables accept it
INSERT INTO routers VALUES ('R2', 'Aero');
INSERT INTO trader_routers VALUES ('ana', 'R2');
"""

LOST_RULE = """
-- traders with two routers on one exchange: the rule no key holds now
SELECT t.trader, r.dex, COUNT(*) AS routers
FROM trader_routers t JOIN routers r ON r.router = t.router
GROUP BY t.trader, r.dex
HAVING COUNT(*) > 1;
"""


def closure(attrs):
    # every column the rules let these columns decide
    got = set(attrs)
    changed = True
    while changed:
        changed = False
        for left, right in FDS:
            if set(left) <= got and right not in got:
                got.add(right)
                changed = True
    return got


def is_superkey(attrs):
    return closure(attrs) == set(COLS)


def candidate_keys():
    # a superkey with nothing extra: drop any column and it stops being one
    keys = []
    for size in range(1, len(COLS) + 1):
        for attrs in itertools.combinations(COLS, size):
            if is_superkey(attrs) and not any(set(k) <= set(attrs) for k in keys):
                keys.append(attrs)
    return keys


def rows(db, sql):
    return db.execute(sql).fetchall()


def main():
    db = sqlite3.connect(":memory:")
    db.executescript(SETUP)
    print("rows:", rows(db, "SELECT * FROM trader_routes"))

    print("superkey trader + dex + router:", is_superkey(COLS))
    print("superkey trader + dex:", is_superkey(("trader", "dex")))
    print("superkey trader + router:", is_superkey(("trader", "router")))
    keys = candidate_keys()
    print("candidate keys:", [" + ".join(k) for k in keys])

    for col in COLS:
        print(f"{col} alone repeats:", rows(db, REPEATS.format(cols=col)))

    for left, right in FDS:
        broken = rows(db, BREAKS_FD.format(left=", ".join(left), right=right))
        print(f"{' + '.join(left)} decides {right}: breaks in data = {broken}")

    prime = {c for k in keys for c in k}
    for left, right in FDS:
        bcnf = is_superkey(left)
        third = bcnf or right in prime
        print(f"rule {' + '.join(left)} -> {right}: determinant is a key = {bcnf}, "
              f"BCNF ok = {bcnf}, 3NF ok = {third}")

    print("R1 = Aero stored", rows(db, "SELECT COUNT(*) FROM trader_routes WHERE router = 'R1'")[0][0], "times")
    # split from the full rows first, so both shapes face the same delete
    db.executescript(SPLIT)
    print("routers:", rows(db, "SELECT * FROM routers ORDER BY router"))
    print("trader_routers:", rows(db, "SELECT * FROM trader_routers ORDER BY trader, router"))

    db.executescript(LOSE_FACT)
    print("one table, after delete, R7 belongs to:", rows(db, ROUTER_DEX))

    db.executescript(DELETE_USE)
    print("split, after delete, R7 belongs to:", rows(db, STILL_THERE))

    db.executescript(ADD_SECOND_ROUTER)
    print("two routers on one exchange:", rows(db, LOST_RULE))


main()
