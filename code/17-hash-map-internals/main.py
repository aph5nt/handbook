"""A hash map with chaining: the lesson's seven keys, the resize, and the attack.

Run: python3 main.py
"""


class ChainedMap:
    def __init__(self, size=8, hash_fn=lambda key: key):
        self.buckets = [[] for _ in range(size)]
        self.hash_fn = hash_fn               # the key's number; here the key itself
        self.count = 0

    def bucket_of(self, key):
        return self.hash_fn(key) % len(self.buckets)  # one hash, one division

    def put(self, key, value):
        chain = self.buckets[self.bucket_of(key)]     # one array jump
        for entry in chain:
            if entry[0] == key:              # the same key again: overwrite
                entry[1] = value
                return
        chain.append([key, value])           # a new key goes on the chain
        self.count += 1
        if self.count / len(self.buckets) > 0.75:     # load factor past the limit
            self.resize(2 * len(self.buckets))

    def resize(self, new_size):
        old = self.buckets
        self.buckets = [[] for _ in range(new_size)]
        for chain in old:                    # touch every key: O(n), once
            for key, value in chain:
                self.buckets[self.bucket_of(key)].append([key, value])  # rehash

    def get(self, key):
        steps = 0
        for k, value in self.buckets[self.bucket_of(key)]:
            steps += 1                       # one compare per entry on the chain
            if k == key:
                return value, steps
        return None, steps

    def show(self):
        return {b: [hex(k) for k, _ in c] for b, c in enumerate(self.buckets) if c}


def main():
    m = ChainedMap()
    for key in (0xa1, 0x3c, 0x72, 0x19, 0xe8, 0x5b):
        m.put(key, "row")
        print(f"put {hex(key)}: {key} mod 8 = {key % 8}, load {m.count} / {len(m.buckets)} = {m.count / len(m.buckets)}")
    print("chains:", m.show())
    print("get 0x19 before the resize:", m.get(0x19))
    print(f"put 0x44: 68 mod 8 = {0x44 % 8}, load 7 / 8 = {7 / 8}: past 0.75, so it resizes")
    m.put(0x44, "row")
    print("buckets now:", len(m.buckets), "chains:", m.show())
    print("get 0x19 after the resize:", m.get(0x19), f" 25 mod 16 = {25 % 16}")

    bad = ChainedMap(size=16, hash_fn=lambda key: 0)  # a weak hash: everything in bucket 0
    for key in (0xa1, 0x3c, 0x72, 0x19, 0xe8, 0x5b, 0x44):
        bad.put(key, "row")
    print("weak hash, 7 keys, 16 buckets:", bad.show())
    print("get 0x44 with a weak hash:", bad.get(0x44))


if __name__ == "__main__":
    main()
