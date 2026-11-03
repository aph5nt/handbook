"""Trees: a binary search tree, a B+ tree with page reads counted, and a min-heap.

Replays the lesson's three stories.
Run: python3 main.py
"""
import bisect
import random


class BSTNode:
    def __init__(self, key):
        self.key, self.left, self.right = key, None, None


def bst_insert(root, key):
    if root is None:
        return BSTNode(key)                  # an empty spot: the key lands here
    if key < root.key:                       # smaller keys go left
        root.left = bst_insert(root.left, key)
    else:                                    # bigger keys go right
        root.right = bst_insert(root.right, key)
    return root


def bst_search(root, key):
    compares, node = [], root
    while node is not None:                  # one path from the top
        compares.append(node.key)
        if key == node.key:
            return True, compares            # found
        node = node.left if key < node.key else node.right  # drop the other side
    return False, compares


def height(node):
    return 0 if node is None else 1 + max(height(node.left), height(node.right))


class BPlusTree:
    """Built bottom up from sorted keys. A page holds `fanout` keys or children."""

    def __init__(self, n_keys, fanout):
        # A leaf is a range of keys, so 10 million keys need almost no memory.
        self.leaves = [range(s, min(s + fanout, n_keys)) for s in range(0, n_keys, fanout)]
        level = [(leaf[0], leaf) for leaf in self.leaves]
        self.height = 1
        while len(level) > 1:                # stack levels until one root is left
            pages = [level[k:k + fanout] for k in range(0, len(level), fanout)]
            level = [(page[0][0], page) for page in pages]
            self.height += 1
        self.root = level[0][1]

    def search(self, key):
        page, reads = self.root, 1           # reading the root is one page read
        while not isinstance(page, range):
            firsts = [first for first, _ in page]    # binary search inside the page
            page = page[bisect.bisect_right(firsts, key) - 1][1]
            reads += 1                       # going down one level: one more page
        return key in page, reads

    def range_scan(self, key, count):
        at = next(i for i, leaf in enumerate(self.leaves) if key in leaf) if count else 0
        got, reads = 0, 0
        while got < count:                   # walk the linked leaves, left to right
            leaf = self.leaves[at + reads]
            got += sum(1 for k in leaf if k >= key)
            reads += 1
        return reads


def page_splits(keys, capacity):
    """Insert keys into leaf pages only. A full page splits. Returns splits, fill."""
    firsts, pages, splits = [keys[0]], [[]], 0
    for key in keys:
        i = max(bisect.bisect_right(firsts, key) - 1, 0)
        bisect.insort(pages[i], key)
        if len(pages[i]) > capacity:
            at_right_edge = i == len(pages) - 1 and key == pages[i][-1]
            cut = capacity if at_right_edge else len(pages[i]) // 2  # right edge: leave it full
            pages[i], new = pages[i][:cut], pages[i][cut:]
            pages.insert(i + 1, new)
            firsts.insert(i + 1, new[0])
            splits += 1
    return splits, len(keys) / (len(pages) * capacity)


def heap_push(a, x):
    a.append(x)                              # always start at the end
    i, swaps = len(a) - 1, []
    while i > 0:
        parent = (i - 1) // 2
        if a[i] >= a[parent]:                # the heap rule holds: stop
            break
        a[i], a[parent] = a[parent], a[i]    # sift up one level
        swaps.append(list(a))
        i = parent
    return swaps


def heap_pop(a):
    top, last = a[0], a.pop()                # the last item fills the hole
    if a:
        a[0] = last
        i = 0
        while True:                          # sift down
            kids = [c for c in (2 * i + 1, 2 * i + 2) if c < len(a)]
            if not kids:
                break
            small = min(kids, key=lambda c: a[c])    # the smaller child
            if a[i] <= a[small]:             # the heap rule holds: stop
                break
            a[i], a[small] = a[small], a[i]
            i = small
    return top


def main():
    root = None
    for key in (50, 30, 70, 20, 40, 60, 80):
        root = bst_insert(root, key)
    print("find 40:", bst_search(root, 40), " height", height(root))
    chain = None
    for key in (1, 2, 3, 4, 5):
        chain = bst_insert(chain, key)
    print("sorted inserts, find 5:", bst_search(chain, 5), " height", height(chain))

    tree = BPlusTree(10_000_000, 200)
    print("B+ tree, 10 million keys, 200 per page: levels", tree.height)
    print("find 9,507,123: (found, page reads)", tree.search(9_507_123))
    print("the next 1,000 blocks: leaf pages walked", tree.range_scan(9_507_123, 1000))
    print("levels at 200 per page, a billion keys:", 4, "because 200^4 =", f"{200 ** 4:,}")
    rng = random.Random(7)
    uuids = [rng.getrandbits(64) for _ in range(100_000)]
    for name, keys in (("increasing block heights", list(range(100_000))), ("random UUIDs", uuids)):
        splits, fill = page_splits(keys, 100)
        print(f"{name}: {splits:,} page splits, pages {fill:.0%} full")

    a = [3, 7, 5, 9, 8, 6]
    print("heap:", a, " peek:", a[0])
    print("push 2, each swap:", heap_push(a, 2))
    print("pop:", heap_pop(a), "heap now", a)
    print("find 8: read slots", [s for s in range(a.index(8) + 1)], "(the heap only knows its top)")


if __name__ == "__main__":
    main()
