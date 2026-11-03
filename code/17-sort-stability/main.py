"""Stable and unstable sorts on the lesson's six transfers, written out so each compare can be seen.

Run: python3 main.py
"""


def merge_sort(rows, key):
    if len(rows) <= 1:
        return rows
    mid = len(rows) // 2
    left = merge_sort(rows[:mid], key)
    right = merge_sort(rows[mid:], key)
    i = j = 0
    out = []
    while i < len(left) and j < len(right):
        if key(left[i]) <= key(right[j]):        # a tie takes the left row: that is stability
            out.append(left[i])
            i += 1
        else:
            out.append(right[j])
            j += 1
    return out + left[i:] + right[j:]            # whichever side still has rows


def address(row):
    return row[0]


def block(row):
    return row[1]


def two_passes(rows):
    rows = merge_sort(rows, block)
    return merge_sort(rows, address)             # keeps the block order among ties


def quicksort(rows, lo, hi, key, trace=None):
    if lo >= hi:
        return
    pivot = key(rows[hi])                        # the last row is the pivot
    i = lo
    for j in range(lo, hi):
        if key(rows[j]) < pivot:
            rows[i], rows[j] = rows[j], rows[i]
            i += 1
    rows[i], rows[hi] = rows[hi], rows[i]        # a long swap: it can jump over a tie
    if trace is not None:
        trace.append(list(rows))
    quicksort(rows, lo, i - 1, key, trace)
    quicksort(rows, i + 1, hi, key, trace)


def address_then_block(row):
    return (row[0], row[1])                      # the tie-breaker: no two rows are equal now


def quicksort_copy(rows, key):
    rows = list(rows)                            # leave the input alone
    quicksort(rows, 0, len(rows) - 1, key)
    return rows


def main():
    transfers = [("B", 101), ("A", 102), ("B", 103), ("A", 104), ("C", 105), ("A", 106)]
    print("input:", transfers)
    print("merge sort by address:", merge_sort(transfers, address))
    shuffled = [("A", 106), ("C", 105), ("B", 101), ("A", 102), ("B", 103), ("A", 104)]
    print("two passes, block then address:", two_passes(shuffled))
    print("Python's sorted is stable too:", sorted(transfers, key=address))

    rows, trace = list(transfers), []
    quicksort(rows, 0, len(rows) - 1, address, trace)
    print("quicksort, first partition:", trace[0])
    print("quicksort by address:", rows)
    print("A blocks:", [b for a, b in rows if a == "A"], " B blocks:", [b for a, b in rows if a == "B"])
    print("quicksort with the tie-breaker:", quicksort_copy(transfers, address_then_block))


if __name__ == "__main__":
    main()
