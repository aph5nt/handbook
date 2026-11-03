"""Three graph jobs on the lesson's data: topological order, Dijkstra, union-find.

Run: python3 main.py
"""
import heapq
from collections import deque


def topo_order(nodes, edges):
    count = {node: 0 for node in nodes}          # how many each node still waits for
    after = {node: [] for node in nodes}
    for first, then in edges:
        after[first].append(then)
        count[then] += 1
    ready = deque(node for node in nodes if count[node] == 0)
    order = []
    while ready:
        node = ready.popleft()                   # safe to run now
        order.append(node)
        for nxt in after[node]:                  # remove its arrows
            count[nxt] -= 1
            if count[nxt] == 0:
                ready.append(nxt)
    left = [node for node in nodes if node not in order]
    if left:                                     # some never reached 0: a cycle
        return order, left
    return order, []


def dijkstra(edges, source, goal, trace=None):
    dist, parent, done = {source: 0}, {source: None}, set()
    heap = [(0, source)]
    while heap:
        d, node = heapq.heappop(heap)            # the closest frontier node
        if node in done:                         # an old entry: a cheaper one came first
            continue
        done.add(node)                           # final: assumes no negative cost
        if node == goal:
            break
        for nxt, cost in edges.get(node, []):
            if nxt in done:                      # already final: it will not be revisited
                continue
            new = d + cost
            if nxt not in dist or new < dist[nxt]:   # relax: is this route cheaper?
                dist[nxt], parent[nxt] = new, node
                heapq.heappush(heap, (new, nxt))
        if trace is not None:
            trace.append((node, dict(dist), sorted(heap)))
    return dist, parent


class UnionFind:
    def __init__(self, n):
        self.parent, self.rank = list(range(n)), [0] * n

    def find(self, x):
        root = x
        while self.parent[root] != root:         # walk up to the root
            root = self.parent[root]
        while x != root:                         # path compression: point each at the root
            self.parent[x], x = root, self.parent[x]
        return root

    def union(self, a, b):
        ra, rb = self.find(a), self.find(b)
        if ra == rb:                             # one group already
            return
        if self.rank[ra] < self.rank[rb]:        # hang the shorter tree under the taller
            ra, rb = rb, ra
        self.parent[rb] = ra                     # one pointer changes: root to root
        if self.rank[ra] == self.rank[rb]:
            self.rank[ra] += 1

    def union_naive(self, a, b):
        self.parent[self.walk(a)[-1]] = self.walk(b)[-1]   # no rank, no compression

    def walk(self, x):
        path = [x]
        while self.parent[path[-1]] != path[-1]:
            path.append(self.parent[path[-1]])
        return path


def main():
    nodes = ["prices", "telemetry", "EVM", "graph sync"]
    arrows = [("prices", "EVM"), ("EVM", "graph sync")]
    print("order:", topo_order(nodes, arrows))
    print("with a cycle:", topo_order(nodes, arrows + [("graph sync", "prices")]))

    roads = {"S": [("A", 4), ("B", 1)], "B": [("A", 2), ("C", 5)], "A": [("T", 1)], "C": [("T", 3)]}
    trace = []
    dist, parent = dijkstra(roads, "S", "T", trace)
    for node, known, heap in trace:
        print(f"popped {node}: {known} heap {heap}")
    route, at = [], "T"
    while at:
        route.append(at)
        at = parent[at]
    print("route:", route[::-1], "cost", dist["T"])
    negative = {"S": [("A", 2), ("B", 5)], "B": [("A", -4)]}
    print("negative cost, Dijkstra says A =", dijkstra(negative, "S", None)[0]["A"], "but S, B, A costs", 5 - 4)

    w = UnionFind(6)                             # w1 to w6 are numbers 0 to 5
    w.union(0, 1)
    print("union(w1, w2):", w.parent)
    w.union(2, 3)
    print("union(w3, w4):", w.parent)
    w.union(1, 2)
    print("union(w2, w3):", w.parent)
    print("find(w4) before:", w.walk(3))
    w.find(3)
    print("find(w4) after:", w.parent, "next walk:", w.walk(3))
    print("same group, w4 and w2:", w.find(3) == w.find(1), " w4 and w5:", w.find(3) == w.find(4))
    chain = UnionFind(6)
    for i in range(5):
        chain.union_naive(i, i + 1)
    print("no rank, no compression:", chain.parent, "find(w1) hops:", len(chain.walk(0)) - 1)


if __name__ == "__main__":
    main()
