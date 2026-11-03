"""Constrained decoding: replay the lesson "Constrained decoding: only valid tokens can be drawn".

A tiny vocabulary after the prefix {"kind": " , a grammar built from an enum,
a mask, and a content check against the graph's own mark.
Probabilities are the lesson's example. Standard library only. Run: python3 main.py
"""
import json
import random

# The model's next-token list after {"kind": "   (example numbers)
NEXT = {"exchange": 0.523, "mixer": 0.371, "unknown": 0.103,
        "probably": 0.002, "maybe": 0.0006, "a mixer": 0.0004}


def grammar(enum):
    """The tokens that keep the output valid for the schema {kind: one of enum}."""
    return set(enum)


def mask(probs, allowed):
    """Zero every token outside the grammar, then share the weight among the rest."""
    kept = {t: p for t, p in probs.items() if t in allowed}
    total = sum(kept.values())
    return {t: p / total for t, p in kept.items()}


def draw(probs, rng):
    r, acc = rng.random(), 0.0
    for token, p in probs.items():
        acc += p
        if r < acc:
            return token
    return token


def finish(token):
    """Write the rest of the output. An off-grammar token runs on as free text."""
    if token in ("probably", "maybe"):
        token += " a mixer"
    return '{"kind": "' + token + '"}'


def parses(text, enum):
    try:
        return json.loads(text)["kind"] in enum
    except ValueError:
        return False


ENUM = ["exchange", "mixer", "unknown"]
rng = random.Random(7)

print("Tab 1: the mask")
bad = sum(not parses(finish(draw(NEXT, rng)), ENUM) for _ in range(10_000))
print(f"no constraints: {bad} of 10,000 outputs break the schema, e.g. {finish('probably')}")
masked = mask(NEXT, grammar(ENUM))
print("masked list:", ", ".join(f"{t} {p:.1%}" for t, p in masked.items()))
bad = sum(not parses(finish(draw(masked, rng)), ENUM) for _ in range(10_000))
print(f"constrained: {bad} of 10,000 outputs break the schema")
print("step 6 draws the most likely allowed token:", finish(max(masked, key=masked.get)))

print()
print("Tab 2: shape is not truth")
graph_mark = "mixer"                                      # the graph's own mark for this address
mixer_case = {"exchange": 0.55, "mixer": 0.35, "unknown": 0.10}
label = max(mixer_case, key=mixer_case.get)
out = finish(label)
print(f"mixer address: model writes {out}, parses: {parses(out, ENUM)}")
print("content check:", "agrees" if label == graph_mark else f"flagged: graph says {graph_mark.capitalize()}")
two = mask({"exchange": 0.5, "mixer": 0.5, "unknown": 0.0}, grammar(["exchange", "mixer"]))
print(f"enum without unknown, no-evidence address: forced to choose from {sorted(two)}")
three = {"exchange": 0.15, "mixer": 0.15, "unknown": 0.70}
print("enum with unknown back:", finish(max(three, key=three.get)))
