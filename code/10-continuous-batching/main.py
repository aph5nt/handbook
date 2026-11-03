"""Continuous batching: replay the lesson "Continuous batching: no slot waits for the longest answer".

Part 1 runs two batch slots for 12 decode steps, static and continuous, and
prints the same grids as the lesson. A lower-case letter is a prefill step.
Part 2 admits requests into 12 KV cache pages, 2 pages each, with and
without a cap on live requests. Standard library only. Run: python3 main.py
"""

SLOTS, STEPS = 2, 12
REQUESTS = [("A", 12), ("B", 3), ("C", 5), ("D", 4)]   # name, decode tokens


def run(continuous):
    queue = list(REQUESTS)
    slots = [None] * SLOTS                  # each: [name, tokens left, needs prefill]
    rows = [""] * SLOTS
    for step in range(1, STEPS + 1):
        # admit: static only when the whole batch is empty, continuous whenever a slot is free
        batch_empty = all(s is None for s in slots)
        for k in range(SLOTS):
            if slots[k] is None and queue and (continuous or batch_empty):
                name, n = queue.pop(0)
                slots[k] = [name, n, step > 1]   # requests joining later run a prefill step first
        for k in range(SLOTS):
            s = slots[k]
            if s is None:
                rows[k] += "."
            elif s[2]:
                rows[k] += s[0].lower()       # prefill this step
                s[2] = False
            else:
                rows[k] += s[0]
                s[1] -= 1
                if s[1] == 0:
                    slots[k] = None           # finished: the slot is free next step
    return rows, [q[0] for q in queue]


for mode in (False, True):
    rows, waiting = run(mode)
    print("continuous" if mode else "static", rows, "still queued:", waiting)
static_rows, _ = run(False)
print("empty slot-steps in static:", sum(r.count(".") for r in static_rows))

PAGES, PER_REQUEST = 12, 2


def admit(live, name, cap):
    """Admit a request if the cap allows it and pages are free; else evict or queue."""
    used = len(live) * PER_REQUEST
    if cap is not None and len(live) >= cap:
        return live, f"{name} waits in the queue (cap {cap})"
    if used + PER_REQUEST > PAGES:
        victim = live[2]                      # no free page: the server evicts a live request
        live = [r for r in live if r != victim] + [name]
        return live, f"{name} admitted, {victim} evicted and must redo its prefill"
    return live + [name], f"{name} admitted"


print()
for cap in (None, 5):
    live = []
    print("cap:", cap)
    for name in "ABCDEFG":
        live, msg = admit(live, name, cap)
        if name in "DFG":
            print(f"  after {name}: {len(live)} live, {len(live) * PER_REQUEST} of {PAGES} pages; {msg}")
