"""Prefix sums: the lesson's eight blocks, two range queries and one late correction.

Run: python3 main.py
"""


def build(a):
    """prefix[i] is the sum of the first i amounts. prefix[0] is 0."""
    prefix = [0] * (len(a) + 1)  # one more slot than there are blocks
    for i in range(len(a)):
        prefix[i + 1] = prefix[i] + a[i]  # the total before, plus one amount
    return prefix


def range_sum(prefix, l, r):
    """Sum of blocks l to r, both included."""
    # prefix[r + 1] holds blocks 0..r, prefix[l] holds blocks 0..l-1.
    # The difference leaves exactly blocks l..r.
    return prefix[r + 1] - prefix[l]


def naive_sum(a, l, r):
    """What prefix sums replace: add the range up again every time."""
    total = 0
    for i in range(l, r + 1):
        total += a[i]  # one addition per block in the range
    return total


def update(a, prefix, i, v):
    """A late correction to block i. Every total after it moves."""
    delta = v - a[i]
    a[i] = v
    rewritten = 0
    for j in range(i + 1, len(prefix)):
        prefix[j] += delta  # this is the O(n) cost of one update
        rewritten += 1
    return rewritten


def main():
    a = [5, 3, 8, 2, 7, 4, 6, 1]
    prefix = build(a)
    print("amounts:", a)
    print("prefix: ", prefix)
    for l, r in [(2, 5), (3, 6)]:
        got = range_sum(prefix, l, r)
        print(f"blocks {l} to {r}: prefix[{r + 1}] - prefix[{l}] = "
              f"{prefix[r + 1]} - {prefix[l]} = {got}  (check by adding: {naive_sum(a, l, r)})")
    print("1,000 questions of 250 blocks each:",
          f"{1000 * 250:,} additions without prefix sums, {1000 * 2:,} lookups with them")
    n = update(a, prefix, 1, 10)
    print(f"late correction: block 1 from 3 to 10, delta 7, totals rewritten: {n}")
    print("prefix: ", prefix)


if __name__ == "__main__":
    main()
