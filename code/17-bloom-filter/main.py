"""A Bloom filter: the lesson's 16 bits and 3 hashes, using sha256 so every run matches.

Run: python3 main.py
"""
import hashlib
import math


class BloomFilter:
    def __init__(self, m, k):
        self.m, self.k = m, k
        self.bits = [0] * m                  # the filter stores no keys, only bits

    def positions(self, key):
        """k different hash functions: sha256 of the function number plus the key."""
        return [int(hashlib.sha256(f"{i}:{key}".encode()).hexdigest(), 16) % self.m
                for i in range(self.k)]

    def add(self, key):
        for p in self.positions(key):
            self.bits[p] = 1                 # a bit already 1 stays 1

    def check(self, key):
        for p in self.positions(key):
            if self.bits[p] == 0:            # one 0 bit is proof
                return "definitely absent"
        return "maybe present"               # all set: could be other keys' bits

    def ones(self):
        return [p for p, b in enumerate(self.bits) if b]


def false_positive_rate(m, k, n):
    """The textbook estimate for n keys in m bits with k hashes."""
    return (1 - math.exp(-k * n / m)) ** k


def main():
    f = BloomFilter(16, 3)
    for key in ("0xaa", "0x02"):             # keys chosen so the story shows each case
        print(f"add {key}: bits {sorted(f.positions(key))}")
        f.add(key)
    print("bits set:", f.ones())
    print("check 0xaa:", sorted(f.positions("0xaa")), f.check("0xaa"))
    print("check 0x03:", sorted(f.positions("0x03")), f.check("0x03"),
          " zero bit:", [p for p in f.positions("0x03") if not f.bits[p]])
    print("check 0xc8:", sorted(f.positions("0xc8")), f.check("0xc8"), " but it was never added: a false positive")
    tested = [f"0x{i:02x}" for i in range(0x100) if f"0x{i:02x}" not in ("0xaa", "0x02")]
    print("false positives over 254 other keys:", sum(f.check(k) == "maybe present" for k in tested))
    for i in range(10):
        f.add(f"more-{i}")
    print("after ten more keys, bits set:", f.ones(), len(f.ones()), "of 16")
    print("false positives over the same keys:", sum(f.check(k) == "maybe present" for k in tested))
    print("estimate, 12 keys in 16 bits:", round(false_positive_rate(16, 3, 12), 2))
    print("estimate, 10 bits per key and 7 hashes:", round(false_positive_rate(10_000, 7, 1_000), 4))


if __name__ == "__main__":
    main()
