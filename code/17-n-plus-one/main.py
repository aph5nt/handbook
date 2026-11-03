"""The N+1 shape, counted with a real database (sqlite3 in memory).

Every db.query call counts as one network round trip, as it would be
against a database server. Replays the lesson: 10,000 swaps, each needing
its pool's name.
Run: python3 main.py
"""
import random
import sqlite3

N = 10_000


class CountingDB:
    def __init__(self, conn):
        self.conn = conn
        self.trips = 0

    def query(self, sql, args=()):
        self.trips += 1  # one network round trip per call
        return self.conn.execute(sql, args).fetchall()


def report_lazy(db):
    swaps = db.query("SELECT id, pool_id FROM swaps ORDER BY id LIMIT 10000")  # the 1
    names = []
    for swap_id, pool_id in swaps:  # the N: one query per row
        # What an ORM does when a template reads swap.pool.name:
        row = db.query("SELECT name FROM pools WHERE id = ?", (pool_id,))
        names.append(row[0][0])
    return names


def report_join(db):
    rows = db.query("SELECT s.id, p.name FROM swaps s JOIN pools p"
                    " ON p.id = s.pool_id ORDER BY s.id LIMIT 10000")
    return [name for _, name in rows]  # one round trip for every row


def report_batched(db, page=1000):
    swaps = db.query("SELECT id, pool_id FROM swaps ORDER BY id LIMIT 10000")
    ids = sorted({pool_id for _, pool_id in swaps})
    names = {}
    for k in range(0, len(ids), page):  # pages of 1,000 keys
        chunk = ids[k:k + page]
        marks = ",".join("?" * len(chunk))
        sql = f"SELECT id, name FROM pools WHERE id IN ({marks})"
        for pool_id, name in db.query(sql, chunk):
            names[pool_id] = name
    return [names[pool_id] for _, pool_id in swaps]


class DataLoader:
    """GraphQL style: resolvers ask one key at a time, the loader sends one batch."""

    def __init__(self, db):
        self.db, self.wanted = db, []

    def load(self, pool_id):
        self.wanted.append(pool_id)  # only remember the key during this tick

    def dispatch(self):  # end of the tick: one query for every key asked
        keys = sorted(set(self.wanted))
        marks = ",".join("?" * len(keys))
        return dict(self.db.query(f"SELECT id, name FROM pools WHERE id IN ({marks})", keys))


def setup():
    conn = sqlite3.connect(":memory:")
    conn.execute("CREATE TABLE pools (id INTEGER PRIMARY KEY, name TEXT)")
    conn.execute("CREATE TABLE swaps (id INTEGER PRIMARY KEY, pool_id INTEGER)")
    conn.executemany("INSERT INTO pools VALUES (?, ?)", [(i, f"pool-{i}") for i in range(1, N + 1)])
    pool_ids = list(range(1, N + 1))
    random.Random(7).shuffle(pool_ids)  # fixed seed: the same run every time
    conn.executemany("INSERT INTO swaps VALUES (?, ?)", list(enumerate(pool_ids, start=1)))
    return conn


def main():
    conn = setup()
    first = conn.execute("SELECT pool_id FROM swaps ORDER BY id LIMIT 3").fetchall()
    print("swaps 1, 2, 3 point at pools", [p for (p,) in first])
    for name, fn in [("lazy, one query per row", report_lazy),
                     ("one join", report_join),
                     ("batched, pages of 1,000 keys", report_batched)]:
        db = CountingDB(conn)
        names = fn(db)
        print(f"{name}: {len(names):,} rows, {db.trips:,} round trips,"
              f" at about 1 ms each: about {db.trips / 1000:.1f} s of waiting")
    db = CountingDB(conn)
    swaps = db.query("SELECT id, pool_id FROM swaps ORDER BY id LIMIT 10000")
    loader = DataLoader(db)
    for _, pool_id in swaps:
        loader.load(pool_id)
    print(f"dataloader: {len(loader.dispatch()):,} pools, {db.trips} round trips")
    keys = list(range(1, 1_000_001))
    try:
        CountingDB(conn).query(f"SELECT id FROM pools WHERE id IN ({','.join('?' * len(keys))})", keys)
    except sqlite3.Error as err:
        print("one IN with 1,000,000 keys: refused:", err)


if __name__ == "__main__":
    main()
