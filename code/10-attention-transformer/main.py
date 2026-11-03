"""Tiny attention in plain lists: replay the lesson "Attention: each token weighs every earlier token".

"it" compares its query with the key of every earlier token, turns the scores
into weights that add up to 1 (softmax), and mixes the values. Then it counts
the comparisons prefill does for 8, 16 and 32 tokens. Standard library only.
Run: python3 main.py
"""
import math

TOKENS = ["the", "swap", "moved", "tokens", "and", "it"]
# Hand-made 2-number keys and values (a real model learns hundreds of numbers).
KEYS = {"the": [0.0, -0.6], "swap": [1.9, 1.5], "moved": [0.3, 0.2],
        "tokens": [0.2, 0.0], "and": [-1.0, -0.6], "it": [0.0, -0.6]}
VALUES = {"the": [0.0, 0.1], "swap": [1.0, 0.9], "moved": [0.3, 0.2],
          "tokens": [0.2, 0.3], "and": [0.0, 0.0], "it": [0.1, 0.0]}
QUERY_IT = [1.0, 1.0]            # what "it" looks for


def dot(a, b):
    return sum(x * y for x, y in zip(a, b))


def softmax(scores):
    top = max(scores)
    exps = [math.exp(s - top) for s in scores]
    total = sum(exps)
    return [e / total for e in exps]


def attend(query, keys, values):
    d = len(query)
    scores = [dot(query, k) / math.sqrt(d) for k in keys]    # one number per earlier token
    weights = softmax(scores)                                  # they add up to 1
    mixed = [sum(w * v[i] for w, v in zip(weights, values)) for i in range(d)]
    return scores, weights, mixed


print("Tab 1: what does \"it\" attend to?")
keys = [KEYS[t] for t in TOKENS]
values = [VALUES[t] for t in TOKENS]
scores, weights, mixed = attend(QUERY_IT, keys, values)
for t, s, w in zip(TOKENS, scores, weights):
    print(f"  {t:7s} score {s:+.2f}  weight {w:.0%}")
print(f"  weights add up to {sum(weights):.2f}")
print(f"  new vector for it = [{mixed[0]:.2f}, {mixed[1]:.2f}], close to swap's value {VALUES['swap']}")

print()
print("Tab 2: the cost of attention in prefill")
for n in (8, 16, 32):
    comparisons = n * (n + 1) // 2        # every token with itself and every earlier one
    print(f"  {n:2d} tokens: {comparisons} comparisons per layer")
print("  decode, token 9 after 8 cached: 9 comparisons (one row, keys from the KV cache)")
