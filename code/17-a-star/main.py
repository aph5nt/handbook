"""Dijkstra, A* and weighted A* on the lesson's two floors. One function: only the weight changes.

Run: python3 main.py
"""
import heapq

OPEN_FLOOR = ["............", "............", "......#.....", "S.....#....G", "......#.....", "......#.....", "............"]
TRAP_FLOOR = ["............", "......##....", "...#...#....", "S..#...#...G", "...#...#....", "...#........", "............"]


def find(floor, mark):
    return next((x, y) for y, row in enumerate(floor) for x, c in enumerate(row) if c == mark)


def search(floor, weight):
    start, goal = find(floor, "S"), find(floor, "G")

    def h(x, y):                                 # the guess: columns apart plus rows apart
        return abs(x - goal[0]) + abs(y - goal[1])

    dist, parent, done, order = {start: 0}, {}, set(), []
    frontier = [(weight * h(*start), 0, start[1], start[0])]
    while frontier:
        f, g, y, x = heapq.heappop(frontier)     # the lowest f = g + weight * h
        if (x, y) in done:                       # an old entry: skip it
            continue
        done.add((x, y))
        order.append((x, y))                     # one more cell expanded
        if (x, y) == goal:
            break
        for dx, dy in ((1, 0), (0, 1), (-1, 0), (0, -1)):
            nx, ny = x + dx, y + dy
            if not (0 <= ny < len(floor) and 0 <= nx < len(floor[0])) or floor[ny][nx] == "#":
                continue
            if (nx, ny) not in dist or g + 1 < dist[(nx, ny)]:
                dist[(nx, ny)], parent[(nx, ny)] = g + 1, (x, y)
                heapq.heappush(frontier, (g + 1 + weight * h(nx, ny), g + 1, ny, nx))
    steps, at = 0, goal
    while at in parent:                          # walk the parents back
        at, steps = parent[at], steps + 1
    return len(order), steps, order


def main():
    for name, floor, weight in (("Dijkstra, h = 0", OPEN_FLOOR, 0), ("A*, h = Manhattan", OPEN_FLOOR, 1),
                                ("A*, honest guess, trap floor", TRAP_FLOOR, 1), ("weighted A*, 3 x guess, trap floor", TRAP_FLOOR, 3)):
        expanded, steps, order = search(floor, weight)
        print(f"{name}: {expanded} cells expanded, path {steps} steps. First cells: {order[:5]}")
    goal = find(OPEN_FLOOR, "G")
    print("from S, right cell: f =", 1 + abs(1 - goal[0]), " upper cell: f =", 1 + abs(0 - goal[0]) + 1)


if __name__ == "__main__":
    main()
