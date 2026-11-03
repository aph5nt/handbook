"""Embeddings and vector search: replay the lesson "Embeddings turn meaning into distance".

The documents are toy 2D embeddings, the same points the lesson draws.
Search uses distance in that space: exact search, a greedy graph walk
(the bottom layer of HNSW) and an IVF cluster probe.
Standard library only. Run: python3 main.py
"""
import math

DOCS = {  # model A: the positions drawn in the lesson
    "the table that holds exchange trades": (400, 104),
    "liquidity pool events": (300, 70),
    "price feed for tokens": (470, 220),
    "graph sync layers": (230, 175),
    "wallet risk scores": (60, 205),
    "Grafana board conventions": (130, 250),
    "lane reset runbook": (300, 290),
    "alert routes": (120, 325),
}
NAMES = list(DOCS)
EDGES = [(7, 5), (5, 3), (3, 1), (1, 0), (3, 4), (0, 2), (6, 3), (5, 6), (4, 1), (2, 6)]
ORIGIN = (30, 335)  # where the lesson draws the arrows from


def cosine(a, b):
    """The angle between two arrows from the origin: 1 = same direction."""
    a = (a[0] - ORIGIN[0], ORIGIN[1] - a[1])
    b = (b[0] - ORIGIN[0], ORIGIN[1] - b[1])
    return (a[0] * b[0] + a[1] * b[1]) / (math.hypot(*a) * math.hypot(*b))


def check_models(query_model, doc_model):
    """Vectors from two models live in two spaces: never compare them."""
    if query_model != doc_model:
        raise ValueError(f"query from model {query_model}, documents from model {doc_model}: re-embed")


def exact(q, docs):
    """Compare the question with every vector. Correct, and slow at scale."""
    return min(range(len(docs)), key=lambda i: math.dist(q, docs[i])), len(docs)


def greedy_walk(q, docs, edges, entry):
    """HNSW bottom layer: hop to the neighbour closest to q until none is closer."""
    nbrs = {i: set() for i in range(len(docs))}
    for a, b in edges:
        nbrs[a].add(b)
        nbrs[b].add(a)
    path = [entry]
    while True:
        here = path[-1]
        best = min(nbrs[here], key=lambda n: math.dist(q, docs[n]))
        if math.dist(q, docs[best]) >= math.dist(q, docs[here]):
            return path                  # no neighbour is closer: stop here
        path.append(best)


def ivf(q, docs, clusters, nprobe):
    """IVF: compare q with the centroids, search only the nearest nprobe clusters."""
    def centroid(ids):
        return (sum(docs[i][0] for i in ids) / len(ids), sum(docs[i][1] for i in ids) / len(ids))
    order = sorted(clusters, key=lambda ids: math.dist(q, centroid(ids)))
    checked = [i for ids in order[:nprobe] for i in ids]
    return min(checked, key=lambda i: math.dist(q, docs[i])), len(checked)


A = list(DOCS.values())
Q = (430, 128)  # "where do swaps live", embedded by model A
print("Tab 1: find the neighbour")
words = set("where do swaps live".split())
hits = [n for n in NAMES if words & set(n.split())]
print("keyword search, shared words:", len(hits))
c = cosine(Q, A[0])
print(f"cosine(question, exchange trades) = {c:.3f}")
best, n = exact(Q, A)
print(f"exact search: compared {n} of {n}, nearest = {NAMES[best]}")
path = greedy_walk(Q, A, EDGES, entry=7)
print(f"HNSW walk: {' -> '.join(NAMES[i] for i in path)}  (checked {len(path)} of 8)")
CLUSTERS = [[0, 1, 2], [3, 4], [5, 6, 7]]
best, n = ivf(Q, A, CLUSTERS, nprobe=1)
print(f"IVF probe 1 cluster: checked {n} of 8, nearest = {NAMES[best]}")

print()
print("Tab 2: two models, and missed neighbours")
QB = (175, 282)  # the same question embedded by model B
best, _ = exact(QB, A)  # with the model check switched off
print("model-B question against model-A documents:", NAMES[best], "(wrong)")
try:
    check_models("B", "A")
except ValueError as err:
    print("model check on:", err)
B = [(250, 260), (110, 300), (200, 60), (420, 150), (500, 300), (455, 125), (90, 180), (330, 90)]
QB_new = (280, 284)  # model B question, documents re-embedded by model B
best, _ = exact(QB_new, B)
print("after re-embedding with model B:", NAMES[best])
few = [e for e in EDGES if e != (1, 0)]  # one link fewer
path = greedy_walk(Q, A, few, entry=7)
print(f"walk with fewer links stops at: {NAMES[path[-1]]}  (path of {len(path)})")

# Recall on a golden set: ten questions, each with its known right document.
golden = [((410, 110), 0), ((305, 80), 1), ((460, 215), 2), ((235, 180), 3), ((380, 120), 0),
          ((70, 210), 4), ((140, 245), 5), ((295, 285), 6), ((125, 320), 7), ((465, 230), 2)]
found = [greedy_walk(q, A, few, entry=7)[-1] == right for q, right in golden]
print("golden set:", ["found" if f else "miss" for f in found])
print(f"recall = {sum(found)} of {len(found)} = {sum(found) / len(found):.0%}")
found = [greedy_walk(q, A, EDGES, entry=7)[-1] == right for q, right in golden]
print(f"with the link added back: recall = {sum(found)} of {len(found)}")
