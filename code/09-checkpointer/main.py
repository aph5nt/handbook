"""Checkpointer: save the state after every node.

Replays visual/09-checkpointer.html. sqlite3 stands in for Postgres as the
durable store, a dict for the in-memory one. A "restart" drops every Python
object and opens the store again. Run: python3 main.py
"""
import json
import os
import sqlite3
import tempfile


def load_wallet(s):
    return {**s, "wallet": "0x9f2c4e7a1b3d5f6e8a0c2b4d6f8e0a1c3b5d0071"}


def trace_funders(s):
    return {**s, "funders": ["Funder A", "Funder B"]}


def write_report(s):
    return {**s, "report": f"{len(s['funders'])} funders, approved by {s['approved']}"}


NODES = [("load wallet", load_wallet), ("trace funders", trace_funders),
         ("human approval", None), ("write report", write_report)]


class SqliteStore:
    def __init__(self, path):
        self.db = sqlite3.connect(path)
        self.db.execute("CREATE TABLE IF NOT EXISTS cp (thread TEXT, n INTEGER, state TEXT, next INTEGER)")

    def save(self, thread, state, nxt):
        n = self.db.execute("SELECT COUNT(*) FROM cp WHERE thread=?", (thread,)).fetchone()[0] + 1
        self.db.execute("INSERT INTO cp VALUES (?,?,?,?)", (thread, n, json.dumps(state), nxt))
        self.db.commit()
        return n

    def get(self, thread, n=None):
        q = "SELECT state, next FROM cp WHERE thread=?" + (" AND n=?" if n else " ORDER BY n DESC LIMIT 1")
        row = self.db.execute(q, (thread, n) if n else (thread,)).fetchone()
        return (json.loads(row[0]), row[1]) if row else None


class MemoryStore:
    def __init__(self):
        self.rows = {}

    def save(self, thread, state, nxt):
        self.rows.setdefault(thread, []).append((state, nxt))
        return len(self.rows[thread])

    def get(self, thread, n=None):
        rows = self.rows.get(thread)
        return None if not rows else rows[(n or len(rows)) - 1]


# panel:start
def run(thread, store, start=None):                       # @1
    state, nxt = start or store.get(thread) or ({}, 0)    # @2 resume, or begin
    while nxt < len(NODES):                               # @3
        name, fn = NODES[nxt]
        if fn is None:                                    # @4 needs a person
            if not state.get("approved"):                 # @4
                n = store.save(thread, state, nxt)        # @5 interrupt: save, stop
                print(f"  checkpoint {n}: paused at '{name}'")  # @5
                return state                              # @5
            nxt += 1
            continue
        state = fn(state)                                 # @6 run the node
        nxt += 1                                          # @7
        n = store.save(thread, state, nxt)                # @8 a checkpoint per node
        print(f"  checkpoint {n}: saved after '{name}'")  # @8
    return state


def approve(thread, store, who):                          # @9
    found = store.get(thread)                             # @9 the store must survive
    if found is None:                                     # @9
        raise LookupError(f"thread {thread} not found")   # @9
    state, nxt = found
    return run(thread, store, ({**state, "approved": who}, nxt))  # @2


def rewind(thread, store, n):                             # @10
    return store.get(thread, n)                           # @10 load any old checkpoint
# panel:end


def main():
    path = os.path.join(tempfile.mkdtemp(), "cp.db")
    print("-- tab 1: durable store, resume --")
    run("inv-42", SqliteStore(path))
    print("  process restarted overnight: every Python object is gone")
    state = approve("inv-42", SqliteStore(path), "the analyst")
    print("  report:", state["report"])
    print("  rewind to checkpoint 2:", rewind("inv-42", SqliteStore(path), 2)[0]["funders"])

    print("\n-- tab 2: in-memory only --")
    mem = MemoryStore()
    run("inv-42", mem)
    print("  process restarted overnight: the dict is gone too")
    mem = MemoryStore()
    try:
        approve("inv-42", mem, "the analyst")
    except LookupError as e:
        print("  approval:", e)


if __name__ == "__main__":
    main()
