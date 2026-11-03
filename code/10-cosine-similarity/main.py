"""Cosine similarity: replay the lesson "Cosine similarity: direction, not length".

Two-dimensional toy vectors stand in for embeddings (real ones have hundreds
to thousands of numbers). The program measures cosine, ranks the same results
by dot product and by cosine, shows the order flip, then normalises at write
time so both metrics agree. Standard library only. Run: python3 main.py
"""
import math

QUESTION = ("where do swaps live", [300, 140])
DOCS = {
    "long runbook": [340, 210],          # exchange trades, many words: a long arrow
    "short note": [150, 80],             # exchange trades, few words: a short arrow
    "Grafana board conventions": [-100, 215],
    "opposite direction": [-130, -60],
}
THRESHOLD = 0.75                          # example: measured on a golden set


def dot(a, b):
    return sum(x * y for x, y in zip(a, b))


def length(a):
    return math.sqrt(dot(a, a))


def cosine(a, b):
    return dot(a, b) / (length(a) * length(b))


def normalise(a):
    n = length(a)
    return [x / n for x in a]


q = QUESTION[1]
print("Tab 1: the angle")
for name, v in DOCS.items():
    print(f"  {name:27s} length {length(v):5.0f}  cosine {cosine(q, v):+.3f}")

print()
print("Tab 2: mixed metrics")
pair = ["long runbook", "short note"]
by_dot = sorted(pair, key=lambda d: -dot(q, DOCS[d]))
by_cos = sorted(pair, key=lambda d: -cosine(q, DOCS[d]))
print("  index ranks by dot:   ", by_dot, [dot(q, DOCS[d]) for d in by_dot])
print("  code ranks by cosine: ", by_cos, [round(cosine(q, DOCS[d]), 3) for d in by_cos])
print("  order flips, no error:", by_dot != by_cos)

stored = {d: normalise(v) for d, v in DOCS.items()}          # the fix: at write time
qn = normalise(q)
by_dot_n = sorted(pair, key=lambda d: -dot(qn, stored[d]))
print("  after normalising, dot:", by_dot_n, [round(dot(qn, stored[d]), 3) for d in by_dot_n])
print("  metrics agree:", by_dot_n == by_cos)

for d in ["long runbook", "short note", "Grafana board conventions"]:
    c = dot(qn, stored[d])
    print(f"  threshold {THRESHOLD}: {d} {c:.2f} {'kept' if c >= THRESHOLD else 'dropped'}")
