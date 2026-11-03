"""Embeddings: similar meanings sit close.

Replays visual/09-embeddings.html. fake_embed is a lookup of small hand-made
vectors, so the geometry is the point. The cosine is real.
Run: python3 main.py
"""
import math
import re

# Hand-made vectors. The axes loosely mean: swaps, follow mode, resets.
VECTORS = {
    "core_dex_swaps: one row per swap": [0.95, 0.05, 0.0],
    "swap events, stored per pool": [0.9, 0.1, 0.1],
    "price oracle feeds": [0.6, 0.2, 0.2],
    "lane reset procedure": [0.0, 0.1, 0.95],
    "graph sync follow mode": [0.1, 0.95, 0.1],
    "checkpoint rows: core_dex_follow, core_dex_bulk": [0.7, 0.05, 0.3],
    "Where do swaps live?": [0.92, 0.05, 0.05],
    "core_dex_follow": [0.1, 0.9, 0.2],
}
CHUNKS = [t for t in VECTORS if t not in ("Where do swaps live?", "core_dex_follow")]


def fake_embed(text):
    return VECTORS[text]


# panel:start
def cosine(a, b):                                       # @3
    dot = sum(x * y for x, y in zip(a, b))              # @3
    return dot / (math.hypot(*a) * math.hypot(*b))      # @3 the angle, not the length


INDEX = {c: fake_embed(c) for c in CHUNKS}              # @1 same model for all


def vector_search(question, k=3):
    q = fake_embed(question)                            # @2 same model as the index
    ranked = sorted(INDEX, key=lambda c: cosine(q, INDEX[c]), reverse=True)  # @4
    return ranked[:k]                                   # @4 nearest neighbours


def keyword_search(question):                           # @5
    return [c for c in CHUNKS if question in re.findall(r"[a-z_]+", c)]  # @5 exact


def search(question):
    exact = keyword_search(question)                    # @6
    return exact + [c for c in vector_search(question) if c not in exact]  # @6 merge


def reindex_needed(index_model, query_model):           # @7
    return index_model != query_model                   # @7 a new model is a new map
# panel:end


def main():
    print("-- tab 1 --")
    for i, c in enumerate(vector_search("Where do swaps live?"), 1):
        print(f"{i}. {c}")
    print("\n-- tab 2 --")
    print("vector only, top:", vector_search("core_dex_follow")[0])
    rank = vector_search("core_dex_follow", k=len(CHUNKS)).index("checkpoint rows: core_dex_follow, core_dex_bulk") + 1
    print("checkpoint chunk rank:", rank, "of", len(CHUNKS))
    print("keyword + vector, top:", search("core_dex_follow")[0])
    print("re-embed on a model change:", reindex_needed("model-1", "model-2"))


if __name__ == "__main__":
    main()
