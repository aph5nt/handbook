"""The 3rd biggest of eight amounts, two ways: quickselect and a min-heap of size 3.

Run: python3 main.py
"""
import heapq
import random


def top_k(stream, k, trace=None):
    heap = []
    for value in stream:
        if len(heap) < k:                        # not full: everything goes in
            heapq.heappush(heap, value)
        elif value > heap[0]:                    # beats the bar: replace the smallest
            heapq.heapreplace(heap, value)
        if trace is not None:                    # otherwise skipped, with one compare
            trace.append((value, sorted(heap)))
    return sorted(heap, reverse=True)


def top_k_max_heap(stream, k):
    heap = [-value for value in stream]          # stores all n values
    heapq.heapify(heap)
    return [-heapq.heappop(heap) for _ in range(k)], len(heap) + k


def partition(a, lo, hi, p, tally):
    a[p], a[hi] = a[hi], a[p]                    # park the pivot at the end
    pivot, i = a[hi], lo
    for j in range(lo, hi):
        tally[0] += 1
        if a[j] > pivot:                         # bigger goes left: biggest first
            a[i], a[j] = a[j], a[i]
            i += 1
    a[i], a[hi] = a[hi], a[i]                    # the pivot lands in its final place
    return i


def quickselect(a, k, choose, trace=None):
    lo, hi, tally, rounds = 0, len(a) - 1, [0], 0
    while lo < hi:
        rounds += 1
        p = partition(a, lo, hi, choose(lo, hi), tally)
        if trace is not None:
            trace.append((a[p], p, list(a)))
        if p == k:
            break
        if k < p:                                # the answer is left: drop the right for good
            hi = p - 1
        else:                                    # the answer is right: drop the left for good
            lo = p + 1
    return a[k], rounds, tally[0]


def main():
    amounts = [12, 3, 25, 7, 40, 18, 5, 31]
    rng, trace = random.Random(SEED), []
    answer = quickselect(list(amounts), 2, rng.randint, trace)
    for pivot, position, array in trace:
        print(f"pivot {pivot} lands at position {position}: {array}")
    print("3rd biggest, rounds, compares:", answer)
    biggest_first = sorted(amounts, reverse=True)
    print("sorted input, first value as pivot, find the smallest:",
          quickselect(biggest_first, 7, lambda lo, hi: lo))
    steps = []
    print("min-heap of 3:", top_k(amounts, 3, steps))
    for value, heap in steps:
        print(f"  {value} -> {heap}")
    print("max-heap of everything:", top_k_max_heap(amounts, 3), "values stored")


SEED = 34

if __name__ == "__main__":
    main()
