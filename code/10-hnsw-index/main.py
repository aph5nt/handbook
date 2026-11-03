"""HNSW, a small walk: replay the lesson "HNSW: long hops first, short hops last".

Fifteen points on a line stand in for two million chunk vectors. Three layers
are linked as drawn in the lesson. A greedy walk goes from the entry on the
top layer down to the bottom layer. Then a point is deleted and a link is
lost, the walk stops short, and a rebuild repairs it. Memory and recall
numbers are examples. Standard library only. Run: python3 main.py
"""
XS = [60, 110, 170, 220, 280, 330, 390, 440, 500, 560, 610, 670, 720, 780, 830]
LAYERS = {"top": [1, 7, 12], "mid": [1, 4, 7, 10, 12, 14], "bot": list(range(15))}
QUESTION = 650                          # the question's position
NAMES = {11: "exchange trades", 10: "a wrong chunk"}


def link(points):
    """Link each point to its neighbours in the layer (as the lesson draws it)."""
    return {p: [q for q in (points[i - 1] if i else None,
                            points[i + 1] if i + 1 < len(points) else None) if q is not None]
            for i, p in enumerate(points)}


def build(deleted=()):
    return {name: link([p for p in pts if p not in deleted]) for name, pts in LAYERS.items()}


def walk(graph, entry=1):
    """Greedy: move to the closest neighbour until none is closer, then drop a layer."""
    dist = lambda p: abs(XS[p] - QUESTION)
    computed, path, here = {entry}, [("top", entry)], entry
    for layer in ["top", "mid", "bot"]:
        if path[-1][0] != layer:
            path.append((layer, here))
        while True:
            nbrs = graph[layer].get(here, [])
            computed.update(nbrs)
            best = min(nbrs, key=dist, default=here)
            if dist(best) >= dist(here):
                break
            here = best
            path.append((layer, here))
    return here, path, computed


print("Tab 1: the layered walk")
graph = build()
found, path, computed = walk(graph)
exact = min(range(len(XS)), key=lambda p: abs(XS[p] - QUESTION))   # brute force: all 15 distances
print("  path:", " -> ".join(f"{l}:{p}" for l, p in path))
print(f"  found point {found} ({NAMES.get(found)}), distance {abs(XS[found] - QUESTION)}")
visited = {p for _, p in path}
print(f"  visited {len(visited)} points, computed {len(computed)} distances, out of {len(XS)}")

print()
print("Tab 2: memory, wear and the failure")
N, DIMS, M = 2_000_000, 384, 16         # example index: 384 float32 numbers, M links per layer
vec_bytes = N * DIMS * 4
link_bytes = N * (2 * M * 4 + M * 4 / (M - 1))   # bottom keeps 2M ids, upper layers about 1/(M-1)
print(f"  vectors {vec_bytes / 1e9:.2f} GB, links {link_bytes / 1e9:.2f} GB "
      f"(+{link_bytes / vec_bytes:.0%})")

worn = build(deleted={9})
worn["bot"][10] = [p for p in worn["bot"][10] if p != 11]   # the link beside it is lost too
worn["bot"][11] = [p for p in worn["bot"][11] if p != 10]
found, path, _ = walk(worn)
print("  worn path:", " -> ".join(f"{l}:{p}" for l, p in path))
print(f"  stops at point {found} ({NAMES.get(found)}): no neighbour looks closer")

rebuilt = build(deleted={9})            # the fix: rebuild links from the points that remain
found, path, _ = walk(rebuilt)
print(f"  after rebuild: point {found} ({NAMES.get(found)})")

RECALL = {16: 0.80, 32: 0.90, 64: 0.96, 128: 0.97, 256: 0.97}   # example golden-set recall
efs = sorted(RECALL)
pick = next(e for e, nxt in zip(efs, efs[1:]) if RECALL[nxt] - RECALL[e] < 0.02)
print("  recall by ef:", {e: f"{r:.0%}" for e, r in RECALL.items()}, "-> pick ef", pick)
