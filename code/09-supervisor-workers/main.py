"""Supervisor and workers: briefs out, proof back.

Replays visual/09-supervisor-workers.html. Workers are threads with fake
results. The verifier really re-runs a fake test table. Run: python3 main.py
"""
from concurrent.futures import ThreadPoolExecutor

TASKS = {"A": "migration", "B": "dashboard", "C": "parser fix"}
# Fake test results per branch: what the test command really returns when re-run.
TESTS = {"A": 0, "B": 0, "C": 0, "B-broken": 2}


def worker(name, broken_claim=False, c_blocked=True):
    if name == "C" and c_blocked:
        return {"worker": name, "status": "BLOCKED", "why": "the table name in the brief does not exist"}
    if name == "B" and broken_claim:
        return {"worker": name, "status": "DONE", "command": None, "branch": "B-broken"}
    return {"worker": name, "status": "DONE", "command": "bash ci/t2.sh", "branch": name}


# panel:start
def write_brief(name, task):                                 # @2
    return {"goal": task, "files": f"{task}/", "ruled_out": "..."}  # @2 all it will know


def verify(report):                                          # @5
    failures = TESTS[report["branch"]]                       # @5 run it ourselves
    return failures == 0, failures                           # @5


def supervise(tasks, broken_claim=False, verifier=True, c_blocked=True):
    briefs = {n: write_brief(n, t) for n, t in tasks.items()}  # @1,2 no common file
    with ThreadPoolExecutor(3) as pool:                      # @3 one worktree each
        reports = list(pool.map(lambda n: worker(n, broken_claim, c_blocked), briefs))  # @3,4
    merged, asks = [], []
    for r in reports:
        if r["status"] == "BLOCKED":                         # @7
            asks.append(f"{r['worker']}: {r['why']}")        # @7 one short question
            continue
        if verifier:                                         # @5
            ok, failures = verify(r)                         # @5
            if not ok:                                       # @6
                print(f"  {r['worker']} sent back: {failures} tests fail")  # @6
                continue                                     # @6 nothing reaches main
        merged.append(r["worker"])                           # @8
    return merged, asks
# panel:end


def main():
    print("-- tab 1 --")
    merged, asks = supervise(TASKS)
    print("  merged:", merged, "| ask:", asks)
    print("\n-- tab 2 --")
    merged, _ = supervise(TASKS, broken_claim=True, verifier=False, c_blocked=False)
    print(f"  no verifier, merged: {merged}, main has {TESTS['B-broken']} failing tests")
    merged, _ = supervise(TASKS, broken_claim=True, c_blocked=False)
    print("  verifier, merged:", merged)
    merged, _ = supervise(TASKS, c_blocked=False)
    print("  after the fix, merged:", merged)


if __name__ == "__main__":
    main()
