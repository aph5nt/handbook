"""Amortized cost: a growable array that counts every write.

Replays the lesson: 17 appends that grow by doubling, then 16 appends
that grow by a fixed step of 2 slots.
Run: python3 main.py
"""


class GrowArray:
    def __init__(self, room, grow):
        self.slots = [None] * room  # the memory we own
        self.n = 0                  # how many slots hold an item
        self.grow = grow            # the rule: old room -> new room
        self.writes = 0             # every item written, new or copied

    def append(self, item):
        copied = 0
        if self.n == len(self.slots):          # full: no room for the new item
            bigger = [None] * self.grow(len(self.slots))
            for k in range(self.n):            # copy every old item across
                bigger[k] = self.slots[k]
                copied += 1
            self.slots = bigger
        self.slots[self.n] = item              # write the new item
        self.n += 1
        self.writes += copied + 1
        return copied + 1                      # this append's bar height


def double(room):
    return room * 2  # a factor: copies get rarer as they get bigger


def fixed_step(room):
    return room + 2  # a fixed step: a copy every 2 appends, forever


def amortized(arr):
    return arr.writes / arr.n  # total work divided by the number of appends


def presized(n):
    return GrowArray(n, double)  # room for n up front: no copy ever


def run(name, room, grow, appends):
    arr = GrowArray(room, grow)
    bars = [arr.append(i) for i in range(1, appends + 1)]
    return arr, bars


def main():
    arr, bars = run("doubling", 1, double, 17)
    print("doubling, bar per append:", bars)
    print("after 16 appends: writes", sum(bars[:16]), "average", round(sum(bars[:16]) / 16, 2))
    print("append 17 alone does", bars[16], "writes: it copies all 16 items")
    arr, bars = run("fixed step", 2, fixed_step, 16)
    print("fixed step 2, bar per append:", bars)
    print("after 16 appends: writes", arr.writes, "average", amortized(arr))
    big = presized(17)
    print("pre-sized for 17: bars", [big.append(i) for i in range(1, 18)])
    n, step = 1_000_000, 2
    print(f"a million appends with step 2 copy about n*n/(2*step) = {n * n // (2 * step):,} items")


if __name__ == "__main__":
    main()
