"""Divide and conquer: merge sort on the lesson's eight amounts, counting the work.

Run: python3 main.py
"""
import math

work = 0        # items written by merges, plus items copied by splits


def merge(left, right):
    """Two sorted lists in, one sorted list out: keep taking the smaller head."""
    global work
    out, i, j = [], 0, 0
    while i < len(left) and j < len(right):
        if left[i] <= right[j]:             # compare the two heads
            out.append(left[i])             # <= keeps equal items in order: stable
            i += 1
        else:
            out.append(right[j])
            j += 1
    out += left[i:] + right[j:]             # one side is empty, the other is sorted
    work += len(out)                        # one write per item at this merge
    return out


def merge_sort(items, depth=0, log=None):
    if len(items) <= 1:                     # base case: one item is already sorted
        return items
    mid = len(items) // 2                   # divide: pick the middle
    left = merge_sort(items[:mid], depth + 1, log)
    right = merge_sort(items[mid:], depth + 1, log)
    out = merge(left, right)                # conquer: combine two sorted halves
    if log is not None:
        log.setdefault(depth, []).append((left, right, out))
    return out


def peel_sort(items, depth=0):
    """The failure: split off one item each time. Same merge, far more levels."""
    if len(items) <= 1:
        return items, depth
    rest, deepest = peel_sort(items[1:], depth + 1)  # an uneven split
    return merge(items[:1], rest), deepest


def main():
    global work
    amounts = [38, 27, 43, 3, 9, 82, 10, 21]
    log = {}
    print("sorted:", merge_sort(amounts, log=log), "work:", work, "item writes")
    for depth in sorted(log, reverse=True):
        for left, right, out in log[depth]:
            print(f"  level {depth}: merge {left} + {right} -> {out}")
    print("levels of splitting: log2(8) = 3, each level writes 8 items: 3 x 8 =", 3 * 8)
    work = 0
    out, deepest = peel_sort(amounts)
    print("uneven splits:", out, "levels:", deepest, "work:", work, "item writes")
    for n in (1_000, 1_000_000):
        print(f"n = {n:,}: even splits about {round(n * math.log2(n)):,} writes,"
              f" uneven about {n * (n + 1) // 2 - 1:,}")


if __name__ == "__main__":
    main()
