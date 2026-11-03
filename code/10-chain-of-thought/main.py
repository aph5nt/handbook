"""Chain of thought and the thinking budget: replay the lesson "Chain of thought: think first, then answer".

The two-hop walk is written out step by step, the way a model that thinks
first writes it. A path check in code then verifies the claim, and an eval
table picks the cheapest thinking budget that passes. Eval numbers and token
counts are examples. Standard library only. Run: python3 main.py
"""
FUNDED_BY = {"W": ["A", "B"], "A": ["M"], "B": ["C"], "M": [], "C": []}
LABELS = {"M": "Mixer"}
BUDGET = 2_000                    # thinking tokens allowed per call (example)
BUDGET_FOR = {"two-hop question": 2_000, "easy balance lookup": 0}   # set per task type
PASS_LINE = 0.90                  # the golden set must pass at least this


def think_two_hops(wallet):
    """Write each step down; each line is context for the next one."""
    steps = []
    hop1 = FUNDED_BY[wallet]
    steps.append(f"Hop 1: funders of {wallet} are {' and '.join(hop1)}")
    hop2 = {f: FUNDED_BY[f] for f in hop1}
    steps.append("Hop 2: " + ", ".join(f"{f} from {' and '.join(src)}" for f, src in hop2.items()))
    for f, sources in hop2.items():
        for s in sources:
            if LABELS.get(s) == "Mixer":
                steps.append(f"{s} is labelled Mixer")
                return steps, f"Yes, through {f}, from {s}"
    return steps, "No"


def mixer_path(wallet, max_hops=2):
    """The check in code: a breadth-first walk over funders, at most max_hops deep."""
    frontier = [[wallet]]
    for _ in range(max_hops):
        nxt = []
        for path in frontier:
            for funder in FUNDED_BY[path[-1]]:
                if LABELS.get(funder) == "Mixer":
                    return path + [funder]
                nxt.append(path + [funder])
        frontier = nxt
    return None


print("Tab 1: think first")
steps, answer = think_two_hops("W")
for line in steps:
    print("  wrote:", line)
print("  answer:", answer)

print()
print("Tab 2: the budget")
for task, used in [("two-hop question", 600), ("easy balance lookup", 1_800)]:
    print(f"  {task}: {used} of {BUDGET} thinking tokens = {used / BUDGET:.0%}, billed as output")
for task, b in BUDGET_FOR.items():
    print(f"  fix: {task} gets a budget of {b}")
claimed = "Yes, through B"                      # the written reasoning that lies
path = mixer_path("W")
print(f"  model wrote: {claimed!r}; code path check finds: {' -> '.join(path)}")
print("  accept the yes on the path evidence, not on the written reason")
evals = {"small budget": (0.92, 1), "large budget": (0.93, 3)}   # (pass rate, relative cost)
passing = {k: v for k, v in evals.items() if v[0] >= PASS_LINE}
pick = min(passing, key=lambda k: passing[k][1])
for k, (p, c) in evals.items():
    print(f"  {k}: pass {p:.0%}, cost {c}x")
print("  keep:", pick)
