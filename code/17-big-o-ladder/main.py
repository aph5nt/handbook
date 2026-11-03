"""The Big-O ladder: one function per rung, each counting its own steps.

Part 1 runs every rung on real lists of fake pool addresses and counts steps.
Part 2 prints the ladder the lesson draws (the formula per rung, per n).
Run: python3 main.py
"""
import math

steps = 0


def tick():
    global steps
    steps += 1


def lookup(table, key):              # O(1): one hash, one slot
    tick()
    return table.get(key)


def binary_search(items, key):       # O(log n): halve the range each look
    lo, hi = 0, len(items) - 1
    while lo <= hi:
        tick()
        mid = (lo + hi) // 2
        if items[mid] == key:
            return mid
        lo, hi = (mid + 1, hi) if items[mid] < key else (lo, mid - 1)
    return -1


def has_dup_pairs(addrs):            # O(n^2): every pair. The bug.
    for i in range(len(addrs)):
        for j in range(i + 1, len(addrs)):
            tick()
            if addrs[i] == addrs[j]:
                return True
    return False


def has_dup_sorted(addrs):           # O(n log n): sort, then neighbours
    s = sorted(addrs)                # about n log2 n compares
    for i in range(1, len(s)):
        tick()
        if s[i] == s[i - 1]:
            return True
    return False


def has_dup_set(addrs):              # O(n): one look per item
    seen = set()
    for a in addrs:
        tick()
        if a in seen:
            return True
        seen.add(a)
    return False


def every_subset(addrs):             # O(2^n): every subset
    for mask in range(1 << len(addrs)):
        tick()


def count(fn, *args):
    global steps
    steps = 0
    fn(*args)
    return steps


def main():
    print("measured steps (no duplicates, so every check runs to the end):")
    for n in (10, 100, 1000):
        addrs = [f"0x{k:040x}" for k in range(n)]
        table = {a: k for k, a in enumerate(addrs)}
        row = {"lookup": count(lookup, table, addrs[-1]),
               "binary": count(binary_search, addrs, addrs[-1]),
               "set": count(has_dup_set, addrs),
               "sorted": count(has_dup_sorted, addrs) + round(n * math.log2(n)),
               "pairs": count(has_dup_pairs, addrs)}
        if n <= 20:
            row["subsets"] = count(every_subset, addrs)
        print(f"  n = {n:>5,}: {row}")
    print("the ladder the lesson draws (formula per rung):")
    rungs = [("O(1)", lambda n: 1), ("O(log n)", math.log2), ("O(n)", lambda n: n),
             ("O(n log n)", lambda n: n * math.log2(n)), ("O(n^2)", lambda n: n * n)]
    for n in (10, 100, 1000, 10_000, 1_000_000, 1_590_000):
        cells = [f"{name} {f(n):.3g}" for name, f in rungs]
        sub = f"2^{n}" if n > 30 else f"{2 ** n:,}"
        print(f"  n = {n:>9,}: " + ", ".join(cells) + f", O(2^n) {sub}")
    print(f"the bug at 1.59 million pools: pairs = n(n-1)/2 = {1_590_000 * 1_589_999 // 2:.3e}"
          f" compares, at about 10^8 per second: {1_590_000 * 1_589_999 / 2 / 1e8 / 3600:.1f} hours")


if __name__ == "__main__":
    main()
