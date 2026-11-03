"""Sampling: replay the lesson "temperature shapes the draw, top-p cuts the tail".

The random numbers are fixed lists, so every run prints the same draws as the
lesson. Standard library only. Run: python3 main.py
"""

LABELS = [("exchange", 0.70), ("mixer", 0.20), ("unknown", 0.10)]
NAMES = [("Tracer", 0.40), ("Scout", 0.25), ("Lens", 0.15),
         ("Trail", 0.10), ("Xq#", 0.06), ("zzz", 0.04)]


def shape(probs, temperature):
    """Temperature: 0 keeps only the top token, higher values flatten."""
    if temperature == 0:
        top = max(probs)
        return [1.0 if p == top else 0.0 for p in probs]
    q = [p ** (1 / temperature) for p in probs]
    total = sum(q)
    return [v / total for v in q]


def top_p(probs, p):
    """Keep the top tokens until they cover p of the mass, cut the rest."""
    kept, total = [], 0.0
    for v in probs:                      # probs are sorted, largest first
        if total >= p - 1e-9:
            kept.append(0.0)             # cut: outside the nucleus
        else:
            kept.append(v)
            total += v
    return [v / total for v in kept]     # the kept tokens share the weight


def draw(probs, u):
    """Pick the token whose slice of [0, 1) holds the random number u."""
    edge = 0.0
    for i, v in enumerate(probs):
        edge += v
        if u < edge:
            return i
    return len(probs) - 1


def runs(cands, probs, us):
    return [cands[draw(probs, u)][0] for u in us]


def pct(probs):
    return ", ".join(f"{v:.0%}" for v in probs)


raw = [p for _, p in LABELS]
print("Tab 1: temperature, the classifier")
print("raw list:", pct(raw))
print("T=0:", pct(shape(raw, 0)), "->", runs(LABELS, shape(raw, 0), [0.5] * 10))
U1 = [0.12, 0.81, 0.33, 0.05, 0.95, 0.60, 0.41, 0.77, 0.20, 0.50]
d1 = runs(LABELS, shape(raw, 1), U1)
print("T=1:", pct(shape(raw, 1)), "->", d1, {k: d1.count(k) for k in dict.fromkeys(d1)})
U2 = [0.10, 0.60, 0.90, 0.30, 0.70, 0.45, 0.85, 0.20, 0.65, 0.40]
d2 = runs(LABELS, shape(raw, 2), U2)
print("T=2:", pct(shape(raw, 2)), "->", d2)

print()
print("Tab 2: top-p and the failure")
names = [p for _, p in NAMES]
cum = 0.0
for n, p in NAMES:
    cum += p
    print(f"  {n:7} {p:.0%}  running sum {cum:.0%}")
kept = top_p(names, 0.9)
print("top-p 0.9 keeps:", pct(kept))
U3 = [0.20, 0.50, 0.10, 0.80, 0.30, 0.95, 0.60, 0.05, 0.40, 0.70]
print("10 runs:", runs(NAMES, kept, U3))

# Near a tie, temperature 0 depends on a hair of floating-point noise.
def pick_t0(scores):
    return max(scores, key=scores.get)   # temperature 0: the top score wins


run1 = {"exchange": 0.460, "mixer": 0.450, "unknown": 0.090}
run2 = {"exchange": 0.455, "mixer": 0.456, "unknown": 0.089}  # another batch
print("near tie, run 1:", pick_t0(run1), " run 2:", pick_t0(run2))

# The fix: the eval runs the case 10 times and scores agreement.
eval_noise = [0, 0, -0.012, 0, 0, 0, -0.011, 0, 0, 0]  # exchange drift per run
labels = []
for noise in eval_noise:
    scores = dict(run1)
    scores["exchange"] += noise
    labels.append(pick_t0(scores))
agree = labels.count("exchange")
print("10 runs at temperature 0:", labels)
print(f"agree with exchange: {agree} of 10 -> a flipping case")
