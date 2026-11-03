"""A trie of exchange names, prefix search, its memory cost, and a radix trie.

Run: python3 main.py
"""


class Node:
    def __init__(self):
        self.kids = {}                       # letter -> child node
        self.end = False                     # does a name end here?


def insert(root, word):
    node = root
    for ch in word:                          # one step per letter
        if ch not in node.kids:              # no child for this letter: create it
            node.kids[ch] = Node()
        node = node.kids[ch]                 # reuse or walk down
    node.end = True                          # mark the end of the name


def lookup(root, word):
    node, steps = root, 0
    for ch in word:
        if ch not in node.kids:              # the path stops: not in the trie
            return False, steps
        node = node.kids[ch]
        steps += 1
    return node.end, steps                   # found only if an end is marked here


def prefix_search(root, prefix):
    node = root
    for ch in prefix:                        # walk to the prefix
        if ch not in node.kids:
            return []
        node = node.kids[ch]
    out, stack = [], [(node, prefix)]
    while stack:                             # then every end in the subtree below
        cur, word = stack.pop()
        if cur.end:
            out.append(word)
        for ch, kid in cur.kids.items():
            stack.append((kid, word + ch))
    return sorted(out)


def count_nodes(node):
    return 1 + sum(count_nodes(k) for k in node.kids.values())


def child_slots(root):
    return (count_nodes(root) - 1) * 26      # every letter node keeps room for 26 children


def radix_nodes(node, is_root=True):
    total = 1 if is_root else 0
    for kid in node.kids.values():
        while len(kid.kids) == 1 and not kid.end:  # a chain: merge it into this edge
            kid = next(iter(kid.kids.values()))
        total += 1 + radix_nodes(kid, False)
    return total


def main():
    root = Node()
    for word in ("aero", "aave", "uni", "up"):
        before = count_nodes(root)
        insert(root, word)
        print(f"insert {word}: {count_nodes(root) - before} new nodes")
    print("lookup up:", lookup(root, "up"), " lookup un:", lookup(root, "un"), " lookup zip:", lookup(root, "zip"))
    print("prefix a:", prefix_search(root, "a"), " prefix u:", prefix_search(root, "u"))
    print(f"letter nodes: {count_nodes(root) - 1}, with 26 child slots each: {child_slots(root)} slots for 4 names")
    print("radix trie nodes, root included:", radix_nodes(root), "against", count_nodes(root))


if __name__ == "__main__":
    main()
