"""An LRU cache built by hand: a map plus a doubly linked list. Replays the lesson's eight steps.

Run: python3 main.py
"""


class Node:
    def __init__(self, key=None, value=None):
        self.key, self.value = key, value
        self.prev = self.next = None


class LRU:
    def __init__(self, capacity):
        self.capacity, self.map = capacity, {}
        self.head, self.tail = Node(), Node()    # two markers, never evicted
        self.head.next, self.tail.prev = self.tail, self.head

    def _unlink(self, node):
        node.prev.next, node.next.prev = node.next, node.prev  # no scan: both neighbours are known

    def _link_at_head(self, node):
        node.prev, node.next = self.head, self.head.next
        self.head.next.prev = node
        self.head.next = node

    def get(self, key):
        node = self.map.get(key)                 # one step, however big the cache
        if node is None:
            return None                          # a miss
        self._unlink(node)
        self._link_at_head(node)                 # this key is now the most recent
        return node.value

    def put(self, key, value):
        if len(self.map) == self.capacity:
            victim = self.tail.prev              # the tail is the least recently used
            self._unlink(victim)
            del self.map[victim.key]             # list and map must both forget it
        node = Node(key, value)
        self._link_at_head(node)
        self.map[key] = node

    def order(self):
        out, node = [], self.head.next
        while node is not self.tail:
            out.append(node.key)
            node = node.next
        return out


def evict_by_scan(last_used):
    """The tempting shortcut: no list, only a time per key. Returns the victim and the checks."""
    victim, checks = None, 0
    for key, used in last_used.items():          # every entry, on every eviction
        checks += 1
        if victim is None or used < last_used[victim]:
            victim = key
    return victim, checks


def main():
    cache = LRU(3)
    for key in "ABC":
        cache.put(key, f"abi {key}")
        print(f"put({key}):", cache.order())
    print("get(A):", cache.get("A"), cache.order())
    cache.put("D", "abi D")
    print("put(D):", cache.order(), "B in the map:", "B" in cache.map)
    print("get(B):", cache.get("B"), cache.order())
    print("get(C):", cache.get("C"), cache.order())
    print("scan on 3 entries:", evict_by_scan({"C": 7, "D": 5, "A": 4}))
    print("scan on a million entries:", evict_by_scan({i: i for i in range(1_000_000)}))


if __name__ == "__main__":
    main()
