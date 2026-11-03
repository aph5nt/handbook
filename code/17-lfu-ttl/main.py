"""Three eviction rules on the lesson's requests: LRU, LFU with decay, and a TTL with jitter.

Run: python3 main.py
"""
import random
from collections import Counter, OrderedDict


class LRUCache:
    def __init__(self, capacity=3):
        self.capacity, self.items = capacity, OrderedDict()

    def get(self, key):
        if key not in self.items:
            return "miss"
        self.items.move_to_end(key)              # now the most recent
        return "hit"

    def put(self, key):
        evicted = None
        if len(self.items) == self.capacity:
            evicted = next(iter(self.items))     # the front is the least recent
            del self.items[evicted]
        self.items[key] = True                   # added at the recent end
        return evicted


def run(cache, requests):
    log = []
    for key in requests:
        answer = cache.get(key)
        evicted = cache.put(key) if answer == "miss" else None
        log.append((key, answer, evicted))
    return log


class LFUCache:
    def __init__(self, capacity=3):
        self.capacity, self.counts = capacity, {}

    def get(self, key):
        if key not in self.counts:
            return "miss"
        self.counts[key] += 1                    # how often, not how lately
        return "hit"

    def put(self, key):
        evicted = None
        if len(self.counts) == self.capacity:
            # min takes the first smallest, and a dict keeps insertion order: a tie goes to the oldest
            evicted = min(self.counts, key=self.counts.get)
            del self.counts[evicted]
        self.counts[key] = 1
        return evicted

    def decay(self):
        for key in self.counts:                  # the fix: old counts fade
            self.counts[key] //= 2


class TTLCache:
    def __init__(self, ttl=60, jitter=0, capacity=3, rng=None):
        self.ttl, self.jitter, self.capacity = ttl, jitter, capacity
        self.items, self.rng = OrderedDict(), rng or random.Random(1)

    def get(self, key, now):
        entry = self.items.get(key)
        if entry is None:
            return "miss"
        if now >= entry[1]:                      # past its expiry
            del self.items[key]
            return "miss"
        self.items.move_to_end(key)
        return entry[0]                          # served as is, even if the real price moved

    def put(self, key, value, now):
        expires = now + self.ttl + self.rng.randint(-self.jitter, self.jitter)
        if key not in self.items and len(self.items) == self.capacity:
            self.items.popitem(last=False)       # size is still LRU's job
        self.items[key] = (value, expires)
        return expires


def expiry_peak(jitter, keys=1000):
    cache = TTLCache(jitter=jitter, capacity=keys)
    seconds = Counter(cache.put(f"key {i}", 1, 0) for i in range(1, keys + 1))
    return max(seconds.values()), min(seconds), max(seconds), cache  # the worst second, first, last


def clock(seconds):
    return f"12:{seconds // 60:02d}:{seconds % 60:02d}"


def main():
    lru = LRUCache()
    run(lru, "ABABC")
    print("LRU after A B A B C:", list(lru.items))
    print("LRU scan:", run(lru, "XYZ"))
    print("LRU then normal traffic:", run(lru, "AB"))

    lfu = LFUCache()
    run(lfu, "ABABC")
    print("LFU after A B A B C:", lfu.counts)
    print("LFU scan:", run(lfu, "XYZ"), lfu.counts)
    print("LFU then A:", run(lfu, "A"), lfu.counts)
    old = LFUCache()
    old.counts = {"A": 1000, "B": 900, "D": 1}
    print("LFU a week later:", run(old, "EDED"), old.counts)
    for _ in range(10):
        old.decay()
    print("after ten decays:", dict(old.counts))

    prices = TTLCache()
    print("ETH expires", clock(prices.put("ETH", 3000, 0)))
    print("BTC expires", clock(prices.put("BTC", 90000, 20)))
    print("read ETH at", clock(30), prices.get("ETH", 30))
    print("read ETH at", clock(65), prices.get("ETH", 65), "stored again, expires", clock(prices.put("ETH", 3010, 65)))
    print("SOL expires", clock(prices.put("SOL", 200, 70)), list(prices.items))

    peak, first, last, _ = expiry_peak(0)
    print(f"1,000 keys, no jitter: {peak} expire in the worst second, at {clock(first)}")
    peak, first, last, spread = expiry_peak(6)
    print("key 1 expires", clock(spread.items["key 1"][1]), " key 2 expires", clock(spread.items["key 2"][1]))
    print(f"1,000 keys, 6 s of jitter: {peak} in the worst second, from {clock(first)} to {clock(last)}")


if __name__ == "__main__":
    main()
