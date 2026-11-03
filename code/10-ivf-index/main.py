"""IVF, k-means and a probe: replay the lesson "IVF: search only the nearest clusters".

Seventeen two-number points stand in for fifty million vectors. k-means finds
three centroids, each vector goes into the list of its nearest centroid, and
a question compares with the centroids first, then searches inside the
nprobe nearest lists. Standard library only. Run: python3 main.py
"""
import math

PTS = [(160, 90), (230, 80), (180, 160), (250, 140), (140, 130), (300, 110),
       (430, 220), (500, 290), (450, 280), (520, 230), (410, 260),
       (650, 80), (720, 140), (700, 70), (660, 150), (750, 100), (595, 140)]
SEEDS = [(200, 120), (470, 250), (690, 110)]      # k = 3 starting centres
QA, QB = (270, 170), (530, 150)


def kmeans(points, centres, rounds=10):
    for _ in range(rounds):
        lists = assign(points, centres)
        centres = [tuple(sum(points[i][d] for i in l) / len(l) for d in (0, 1))
                   for l in lists]
    return centres


def assign(points, centres):
    """The inverted file: one list of vector ids per centroid."""
    lists = [[] for _ in centres]
    for i, p in enumerate(points):
        lists[min(range(len(centres)), key=lambda k: math.dist(p, centres[k]))].append(i)
    return lists


def search(q, centres, lists, nprobe):
    order = sorted(range(len(centres)), key=lambda k: math.dist(q, centres[k]))
    probed = order[:nprobe]                                  # compare with centroids only
    cands = [i for k in probed for i in lists[k]]
    best = min(cands, key=lambda i: math.dist(q, PTS[i]))    # compare inside the lists
    return best, probed, len(cands)


centres = kmeans(PTS, SEEDS)
lists = assign(PTS, centres)
print("Tab 1: cluster and probe")
print("  centroids:", [tuple(round(v) for v in c) for c in centres])
print("  list sizes:", [len(l) for l in lists])
print("  question A to centroids:", [round(math.dist(QA, c)) for c in centres])
best, probed, n = search(QA, centres, lists, nprobe=2)
print(f"  nprobe 2: clusters {[k + 1 for k in probed]}, compared {n} of {len(PTS)}, "
      f"best vector {best} at {math.dist(QA, PTS[best]):.0f}")

print()
print("Tab 2: across the border")
print("  question B to centroids:", [round(math.dist(QB, c)) for c in centres])
truth = min(range(len(PTS)), key=lambda i: math.dist(QB, PTS[i]))
print(f"  true neighbour: vector {truth} at {math.dist(QB, PTS[truth]):.0f}, in cluster "
      f"{next(k + 1 for k, l in enumerate(lists) if truth in l)}")
for nprobe in (1, 2):
    best, probed, n = search(QB, centres, lists, nprobe)
    print(f"  nprobe {nprobe}: clusters {[k + 1 for k in probed]}, compared {n}, "
          f"got vector {best} at {math.dist(QB, PTS[best]):.0f}, right: {best == truth}")

NEW = [(820, 300), (840, 270), (800, 330), (860, 320), (830, 240)]
spread = lambda pts, cs: sum(min(math.dist(p, c) for c in cs) for p in pts) / len(pts)
print(f"  old vectors sit {spread(PTS, centres):.0f} from their centroid on average")
print(f"  new vectors sit {spread(NEW, centres):.0f} from the nearest old centroid")
retrained = kmeans(PTS + NEW, centres + [NEW[0]])          # the fix: retrain, one more list
print(f"  after retraining with k = 4: new vectors sit {spread(NEW, retrained):.0f}")
