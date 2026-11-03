"""The race: four sorts on the same twelve numbers, counting compares and moves.

Run: python3 main.py
"""


class Tally:
    def __init__(self):
        self.compares = self.moves = 0

    def compare(self, result):
        self.compares += 1
        return result

    def swap(self, a, i, j):
        a[i], a[j] = a[j], a[i]
        self.moves += 1

    def place(self, a, k, value):
        a[k] = value
        self.moves += 1


def bubble(a, t):
    for p in range(len(a) - 1):
        swapped = False
        for j in range(len(a) - 1 - p):
            if t.compare(a[j] > a[j + 1]):       # neighbours in the wrong order
                t.swap(a, j, j + 1)
                swapped = True
        if not swapped:                          # a clean pass: stop early
            return


def insertion(a, t):
    for i in range(1, len(a)):
        j = i
        while j > 0 and t.compare(a[j - 1] > a[j]):  # walk it left until it fits
            t.swap(a, j - 1, j)
            j -= 1


def merge(a, lo, hi, t):
    if hi <= lo:
        return
    mid = (lo + hi) // 2
    merge(a, lo, mid, t)
    merge(a, mid + 1, hi, t)
    left, right, i, j = a[lo:mid + 1], a[mid + 1:hi + 1], 0, 0
    for k in range(lo, hi + 1):                  # every item is copied back: one move each
        if j == len(right) or (i < len(left) and t.compare(left[i] <= right[j])):
            t.place(a, k, left[i])
            i += 1
        else:
            t.place(a, k, right[j])
            j += 1


def quick(a, lo, hi, t):
    if lo >= hi:
        return
    pivot, i = a[hi], lo                         # always the last item: the trap
    for j in range(lo, hi):
        if t.compare(a[j] < pivot):
            if i != j:
                t.swap(a, i, j)
            i += 1
    if i != hi:
        t.swap(a, i, hi)
    quick(a, lo, i - 1, t)
    quick(a, i + 1, hi, t)


def race(start):
    runners = (("Bubble", lambda a, t: bubble(a, t)), ("Insertion", lambda a, t: insertion(a, t)),
               ("Merge", lambda a, t: merge(a, 0, len(a) - 1, t)), ("Quick", lambda a, t: quick(a, 0, len(a) - 1, t)))
    for name, run in runners:
        a, t = list(start), Tally()
        run(a, t)
        assert a == sorted(start)
        print(f"  {name:10} compares {t.compares:3}  moves {t.moves:3}  total {t.compares + t.moves:3}")


def main():
    print("shuffled:")
    race([7, 3, 10, 1, 12, 5, 9, 2, 11, 4, 8, 6])
    print("already sorted:")
    race(list(range(1, 13)))
    n = 1_000_000
    print(f"at a million items, n squared / (n log2 n) is about {n / 20:,.0f}")


if __name__ == "__main__":
    main()
