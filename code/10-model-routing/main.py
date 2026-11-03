"""Model routing: replay the lesson "Model routing: the cheapest model that can do it".

Cost units are example numbers, not prices. Standard library only.
Run: python3 main.py
"""

SMALL_COST, BIG_COST = 2, 10              # example units per call
ALLOWED = {"exchange", "mixer", "unknown"}
KNOWN = {"0x4a..ee02", "0x77..30ff"}      # addresses the store holds

def route(request):
    """The router: a rule on the request kind. Labels are easy, traces are hard."""
    return "small" if request["kind"] == "label" else "big"

def check(answer, request):
    """Code, not a model: the label must be allowed and the address real."""
    return answer in ALLOWED and request["address"] in KNOWN

def handle(request, small, big, checked=True):
    """Return (answer, cost, path)."""
    if route(request) == "big":
        return big(request), BIG_COST, "big"
    answer = small(request)
    if not checked or check(answer, request):
        return answer, SMALL_COST, "small"
    return big(request), SMALL_COST + BIG_COST, "small, failed check, escalated to big"

# Example model answers for the lesson's three requests.
SMALL = {"0x4a..ee02": "exchange", "0x77..30ff": "probably a mixer"}
small = lambda r: SMALL.get(r["address"], "the funds went to 0x9f..0071")
big = lambda r: {"0x4a..ee02": "exchange", "0x77..30ff": "mixer"}.get(r["address"], "traced over 4 hops")

print("Tab 1: route, check, escalate")
for r in [{"kind": "label", "address": "0x4a..ee02"},
          {"kind": "trace", "address": "stolen funds"},
          {"kind": "label", "address": "0x77..30ff"}]:
    answer, cost, path = handle(r, small, big)
    print(f"{r['kind']:5} {r['address']:12} -> {path}: {answer!r}, cost {cost}")

print()
print("Tab 2: cost of 10 requests")
print("all to the big model:", 10 * BIG_COST)
for fails in (2, 9):
    total = 10 * SMALL_COST + fails * BIG_COST
    print(f"cascade, {fails} of 10 escalate: 10 x {SMALL_COST} + {fails} x {BIG_COST} = {total}")

r = {"kind": "label", "address": "stolen funds"}   # a trace that the router misread as easy
answer, cost, path = handle(r, small, big, checked=False)
print(f"no check: {path}, answer {answer!r} goes out unchecked")
answer, cost, path = handle(r, small, big, checked=True)
print(f"with the check: {path}, answer {answer!r}")
