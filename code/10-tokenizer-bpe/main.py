"""Toy byte-pair encoding (BPE): replay the lesson "Byte-pair encoding: how text becomes tokens".

Train merges on a tiny word list, encode "the swap moved" and a hex address,
and count a letter with code. Standard library only. Run: python3 main.py
"""
from collections import Counter

# A tiny training pile: word -> how often it appears (example counts).
CORPUS = {"swapped": 30, "swap": 25, "swaps": 20, "the": 40, "moved": 25,
          "moves": 10, "mover": 5}
TARGET_MERGES = 11


def pair_counts(words):
    """Count every neighbour pair, weighted by how often the word appears."""
    counts = Counter()
    for pieces, freq in words.items():
        for a, b in zip(pieces, pieces[1:]):
            counts[(a, b)] += freq
    return counts


def merge(words, pair):
    """Replace every a,b neighbour pair with the single piece a+b."""
    out = {}
    for pieces, freq in words.items():
        new, i = [], 0
        while i < len(pieces):
            if i + 1 < len(pieces) and (pieces[i], pieces[i + 1]) == pair:
                new.append(pieces[i] + pieces[i + 1])
                i += 2
            else:
                new.append(pieces[i])
                i += 1
        out[tuple(new)] = freq
    return out


def train(corpus, n):
    words = {tuple(w): f for w, f in corpus.items()}   # start from single bytes
    merges = []
    for _ in range(n):
        counts = pair_counts(words)
        best, count = max(counts.items(), key=lambda kv: kv[1])  # the most common pair
        merges.append(best)
        words = merge(words, best)
        print(f"merge {len(merges)}: {best[0]} + {best[1]} = {best[0] + best[1]} (count {count})")
    return merges


def encode(word, merges):
    """Apply the same merges, in the same order, to new text."""
    pieces = {tuple(word): 1}
    for pair in merges:
        pieces = merge(pieces, pair)
    return list(next(iter(pieces)))


def token_id(piece, merges):
    """Single bytes keep ids 0-255. Merge number n makes piece id 255 + n."""
    if len(piece) == 1:
        return ord(piece)
    names = [a + b for a, b in merges]
    return 255 + names.index(piece) + 1


print("Tab 1: build the vocabulary")
print("start:", list("swapped"), "=", len("swapped"), "pieces")
merges = train(CORPUS, TARGET_MERGES)
for text in ["swapped", "the swap moved"]:
    tokens = [p for w in text.split() for p in encode(w, merges)]
    ids = [token_id(t, merges) for t in tokens]
    print(f"encode {text!r}: {tokens} = {len(tokens)} tokens, ids {ids}")

print()
print("Tab 2: hex, the heavy case")
addr = "0x9f3c8a10b2c4d5e6f708192a3b4c5d6e1d27b0071"[:42]
pieces = encode(addr, merges)
print(f"address of {len(addr)} characters -> {len(pieces)} pieces: this toy vocabulary has no hex merges")
print("a real vocabulary has some two-character hex pieces: about 20 to 25 tokens")
print("model sees pieces like 0x, 9f, 3c, never letters")
print('code counts the letters: "0x9f3c8a10".count("f") =', "0x9f3c8a10".count("f"))
