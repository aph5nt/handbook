"""Fenwick tree and segment tree: range sums with updates, on the lesson's eight blocks.

Run: python3 main.py
"""


class Fenwick:
    def __init__(self, values):
        self.t = [0] * (len(values) + 1)     # slot 0 is unused, blocks count from 1
        for i, v in enumerate(values, start=1):
            self.add(i, v)

    def add(self, i, delta):
        touched = []
        while i < len(self.t):
            self.t[i] += delta
            touched.append(i)
            i += i & -i                      # add the lowest set bit: the next cover
        return touched

    def prefix(self, i):
        total, read = 0, []
        while i > 0:
            total += self.t[i]
            read.append(i)
            i -= i & -i                      # strip the lowest set bit
        return total, read

    def range_sum(self, lo, hi):             # blocks lo to hi, both included
        return self.prefix(hi)[0] - self.prefix(lo - 1)[0]


class SegmentTree:
    def __init__(self, values, combine):
        self.n, self.combine = len(values), combine
        self.t = [0] * (2 * self.n)          # leaves sit at n..2n-1
        self.t[self.n:] = values
        for i in range(self.n - 1, 0, -1):   # each parent combines its two children
            self.t[i] = combine(self.t[2 * i], self.t[2 * i + 1])
        self.writes = 0

    def update(self, pos, value):            # pos counts from 1
        i = pos - 1 + self.n
        self.t[i] = value
        self.writes += 1
        while i > 1:                         # fix each node on the path to the root
            i //= 2
            self.t[i] = self.combine(self.t[2 * i], self.t[2 * i + 1])
            self.writes += 1

    def query(self, lo, hi):                 # blocks lo to hi, both included
        l, r, result = lo - 1 + self.n, hi + self.n, None
        while l < r:                         # climb, taking whole nodes at the edges
            if l % 2 == 1:
                result = self.t[l] if result is None else self.combine(result, self.t[l])
                l += 1
            if r % 2 == 1:
                r -= 1
                result = self.t[r] if result is None else self.combine(result, self.t[r])
            l //= 2
            r //= 2
        return result


def add(a, b):
    return a + b


def main():
    values = [5, 3, 8, 2, 7, 4, 6, 1]
    f = Fenwick(values)
    print("fenwick slots 1..8:", f.t[1:])
    print("prefix 6 (total, slots read):", f.prefix(6))
    print("prefix 2:", f.prefix(2), " range 3 to 6:", f.range_sum(3, 6))
    print("block 2 from 3 to 10: add 7, slots changed", f.add(2, 7))
    print("fenwick slots 1..8:", f.t[1:])

    s = SegmentTree(values, add)
    print("segment tree: blocks 3 to 6 sum", s.query(3, 6))
    s.update(2, 10)
    print("after block 2 = 10: nodes written", s.writes, " root", s.t[1], " 1 to 4:", s.t[2], " 1 to 2:", s.t[4])
    low = SegmentTree([5, 10, 8, 2, 7, 4, 6, 1], min)
    print("min tree: lowest in blocks 3 to 6:", low.query(3, 6))
    s.writes = 0
    for pos in range(1, 9):                  # the failure: range add, one leaf at a time
        s.update(pos, s.t[pos - 1 + s.n] + 5)
    print("add 5 to all 8 blocks one leaf at a time: nodes written", s.writes)


if __name__ == "__main__":
    main()
