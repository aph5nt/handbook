"""BFS and DFS on the lesson's money trail: from the victim to the exchange.

Run: python3 main.py
"""
from collections import deque

EDGES = {
    "victim": ["mule A", "mule B"],
    "mule A": ["mule C", "mule D"],
    "mule B": ["exchange"],
    "mule C": ["exchange"],
    "mule D": ["mule F"],
    "mule F": [],
    "exchange": [],
}


def path_from(parent, node):
    path = []
    while node is not None:                      # follow the parent links back
        path.append(node)
        node = parent[node]
    return path[::-1]


def bfs(edges, start, goal, cap=None):
    queue = deque([start])
    parent = {start: None}                       # this is also the visited set
    while queue:
        node = queue.popleft()                   # first in, first out: nearest first
        if node == goal:
            return path_from(parent, node), len(queue)
        for nxt in edges[node][:cap]:            # cap: at most this many links per node
            if nxt not in parent:                # found already: never queue it twice
                parent[nxt] = node
                queue.append(nxt)
    return None, 0


def dfs(edges, start, goal):
    stack = [start]
    parent = {start: None}
    while stack:
        node = stack.pop()                       # last in, first out: newest first
        if node == goal:
            return path_from(parent, node)
        for nxt in reversed(edges[node]):        # so the first edge ends on top
            if nxt not in parent:
                parent[nxt] = node
                stack.append(nxt)
    return None


def dfs_recursive(edges, node, goal, depth=1):
    if node == goal:
        return depth
    return dfs_recursive(edges, edges[node][0], goal, depth + 1)  # one call frame per hop


def main():
    path, _ = bfs(EDGES, "victim", "exchange")
    print("BFS:", path, f"{len(path) - 1} hops")
    path = dfs(EDGES, "victim", "exchange")
    print("DFS:", path, f"{len(path) - 1} hops")

    wide = dict(EDGES)
    wide["exchange"] = [f"customer {i}" for i in range(1_000_000)]
    for i in range(1_000_000):
        wide[f"customer {i}"] = []
    print("hot wallet, no cap: peak queue",
          peak_queue(wide, None))
    print("hot wallet, cap of 100: peak queue", peak_queue(wide, 100))

    chain = {i: [i + 1] for i in range(100_000)}
    chain[100_000] = []
    print("100,000 hops, explicit stack:", len(dfs(chain, 0, 100_000)) - 1, "hops")
    try:
        dfs_recursive(chain, 0, 100_000)
    except RecursionError as err:
        print("100,000 hops, recursion:", type(err).__name__)


def peak_queue(edges, cap):
    queue, seen, peak = deque(["victim"]), {"victim"}, 1
    while queue:
        for nxt in edges[queue.popleft()][:cap]:
            if nxt not in seen:
                seen.add(nxt)
                queue.append(nxt)
        peak = max(peak, len(queue))
    return peak


if __name__ == "__main__":
    main()
