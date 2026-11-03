"""Binary search against a scan, on the lesson's sixteen block heights.

Run: python3 main.py
"""

SORTED = [3, 8, 12, 17, 21, 25, 30, 34, 41, 47, 52, 58, 63, 70, 77, 85]
BROKEN = [3, 8, 12, 63, 21, 25, 30, 34, 41, 47, 52, 58, 17, 70, 77, 85]


def binary_search(items, target, log=None):
    """items must be sorted. Returns (index or -1, compares)."""
    low, high = 0, len(items) - 1      # everything in low..high may hold target
    compares = 0
    while low <= high:                 # an empty range means: not there
        mid = (low + high) // 2        # the middle slot, rounded down
        compares += 1
        if log is not None:
            log.append((low, high, mid, items[mid]))
        if items[mid] == target:
            return mid, compares       # found
        if items[mid] < target:
            low = mid + 1              # target is bigger: drop mid and the left
        else:
            high = mid - 1             # target is smaller: drop mid and the right
    return -1, compares                # no error, just "not found"


def linear_search(items, target):
    """No order needed. Returns (index or -1, compares)."""
    compares = 0
    for k, value in enumerate(items):  # check each slot in turn
        compares += 1
        if value == target:
            return k, compares
    return -1, compares


def main():
    log = []
    print("binary search for 63:", binary_search(SORTED, 63, log))
    for low, high, mid, value in log:
        side = "found" if value == 63 else ("go right" if value < 63 else "go left")
        print(f"  low {low:2} high {high:2} mid ({low} + {high}) // 2 = {mid:2}: slot holds {value}, {side}")
    print("linear search for 63:", linear_search(SORTED, 63))
    for n in (21_000_000, 1_000_000_000):
        heights = range(n)  # a sorted range: indexing it costs O(1), so this really runs
        print(f"{n:,} sorted heights, worst case:", binary_search(heights, n - 1)[1], "compares")
    log = []
    print("unsorted data, binary search for 63:", binary_search(BROKEN, 63, log), "but 63 is at slot", BROKEN.index(63))
    for low, high, mid, value in log:
        print(f"  low {low:2} high {high:2} mid {mid:2}: slot holds {value}")


if __name__ == "__main__":
    main()
