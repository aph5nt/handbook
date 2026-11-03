"""A skip list: the lesson's eight volumes, a search with moves counted, an insert, a flat list.

Run: python3 main.py
"""
import random


class Node:
    def __init__(self, key, level):
        self.key = key
        self.next = [None] * (level + 1)     # one forward pointer per level


def coin_level(rng, top):
    level = 0
    while level < top and rng.random() < 0.5:  # heads: join the level above
        level += 1
    return level                             # tails: stop here


class SkipList:
    def __init__(self, top=2):
        self.top = top
        self.head = Node(None, top)          # the head is on every level

    def search(self, key, start=None):
        node, moves, drops = self.head, [], 0
        start = self.top if start is None else start
        for level in range(start, -1, -1):   # start high, drop one level at a time
            while node.next[level] and node.next[level].key < key:
                node = node.next[level]      # the next node is smaller: move right
                moves.append(node.key)
            nxt = node.next[level]
            if nxt and nxt.key == key:       # the next node is the target
                moves.append(key)
                return True, moves, drops
            drops += 1                       # too big or missing: drop a level
        return False, moves, drops

    def insert(self, key, level):
        update, node = [self.head] * (self.top + 1), self.head
        for lv in range(self.top, -1, -1):   # the last smaller node on each level
            while node.next[lv] and node.next[lv].key < key:
                node = node.next[lv]
            update[lv] = node
        new = Node(key, level)
        for lv in range(level + 1):          # only nearby pointers change
            new.next[lv] = update[lv].next[lv]
            update[lv].next[lv] = new

    def keys_on(self, level):
        out, node = [], self.head.next[level]
        while node:
            out.append(node.key)
            node = node.next[level]
        return out


def main():
    s = SkipList()
    for key, level in ((3, 0), (9, 1), (14, 0), (21, 2), (30, 0), (38, 1), (45, 2), (52, 0)):
        s.insert(key, level)
    print("level 2:", s.keys_on(2), " level 1:", s.keys_on(1), " level 0:", s.keys_on(0))
    print("search 38 on level 0 only:", s.search(38, start=0))
    print("search 38 from the top:", s.search(38))
    level = coin_level(random.Random(SEED), 2)
    print(f"insert 33: coins with seed {SEED} give level", level)
    s.insert(33, level)
    print("level 1 now:", s.keys_on(1), " level 0 now:", s.keys_on(0))

    flat = SkipList()
    for key in (3, 9, 14, 21, 30, 38, 45, 52):
        flat.insert(key, 0)                  # every coin came up tails
    print("flat list, search 38:", flat.search(38))

    rng = random.Random(7)
    big = SkipList(top=16)
    for key in range(100_000):
        big.insert(key, coin_level(rng, 16))
    found, moves, drops = big.search(77_777)
    print(f"100,000 keys, search 77,777: {len(moves)} moves right and {drops} drops. A sorted list walks 77,778.")


SEED = 1

if __name__ == "__main__":
    main()
