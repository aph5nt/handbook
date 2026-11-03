"""The lesson's five wallets as an adjacency list and as a matrix, counting reads and memory.

Run: python3 main.py
"""

NODES = ["W", "A", "B", "C", "X"]
EDGES = [("W", "A"), ("W", "B"), ("A", "C"), ("B", "X"), ("C", "X")]


def build_list(nodes, edges):
    adj = {node: [] for node in nodes}           # one row per address
    for sender, receiver in edges:
        adj[sender].append(receiver)             # one entry per transfer
    return adj


def list_neighbours(adj, node):
    return adj[node], len(adj[node])             # reads = the size of this row


def list_has_edge(adj, a, b):
    reads = 0
    for receiver in adj[a]:                      # scan one row only
        reads += 1
        if receiver == b:
            return True, reads
    return False, reads


def list_memory(adj):
    return len(adj) + sum(len(row) for row in adj.values())   # V + E


def expand(adj, node, cap, exchanges):
    if node in exchanges:                        # a known hub: stop here and say so
        return [], "exchange, not expanded"
    return adj[node][:cap], "expanded"           # never more than cap links


def build_matrix(nodes, edges):
    index = {node: i for i, node in enumerate(nodes)}
    grid = [[0] * len(nodes) for _ in nodes]     # V by V cells, even with no transfers
    for sender, receiver in edges:
        grid[index[sender]][index[receiver]] = 1
    return grid, index


def matrix_has_edge(grid, index, a, b):
    return grid[index[a]][index[b]] == 1         # one read, however big the graph


def matrix_neighbours(grid, index, nodes, node):
    found, reads = [], 0
    for column, receiver in enumerate(nodes):    # the whole row, every time
        reads += 1
        if grid[index[node]][column] == 1:
            found.append(receiver)
    return found, reads


def matrix_memory(addresses):
    return addresses * addresses                 # V squared cells


def main():
    adj = build_list(NODES, EDGES)
    print("list:", adj)
    print("neighbours of W:", list_neighbours(adj, "W"))
    print("did A send to X:", list_has_edge(adj, "A", "X"))
    print("stored:", list_memory(adj))
    adj["X"] = [f"customer {i}" for i in range(2_000_000)]
    print("X has", len(adj["X"]), "neighbours. Expand with a cap and no mark:", len(expand(adj, "X", 100, set())[0]))
    print("with X marked as an exchange:", expand(adj, "X", 100, {"X"}))

    grid, index = build_matrix(NODES, EDGES)
    for node, row in zip(NODES, grid):
        print(" ", node, row)
    print("did A send to C:", matrix_has_edge(grid, index, "A", "C"), "1 read")
    print("neighbours of W:", matrix_neighbours(grid, index, NODES, "W"))
    zeros = sum(row.count(0) for row in grid)
    print("cells:", matrix_memory(5), "zeros:", zeros)
    cells = matrix_memory(50_000_000)
    print(f"50 million addresses: {cells:.1e} cells, {cells / 8 / 1e12:,.1f} terabytes at one bit each")


if __name__ == "__main__":
    main()
