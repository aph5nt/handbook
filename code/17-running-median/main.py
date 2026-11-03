"""The running median with two heaps. Python's heapq is a min-heap, so the low half stores negated values.

Run: python3 main.py
"""
import heapq


class RunningMedian:
    def __init__(self):
        self.low, self.high = [], []             # low: max-heap by negation, high: min-heap

    def _push(self, x):
        if not self.low or x <= -self.low[0]:    # rule one: no bigger than the low top
            heapq.heappush(self.low, -x)
        else:
            heapq.heappush(self.high, x)

    def add(self, x):
        self._push(x)
        if len(self.low) > len(self.high) + 1:   # rule two: low may hold one more
            heapq.heappush(self.high, -heapq.heappop(self.low))
        elif len(self.high) > len(self.low):     # but never fewer
            heapq.heappush(self.low, -heapq.heappop(self.high))

    def add_broken(self, x):
        self._push(x)                            # the bug: no size check

    def median(self):
        if len(self.low) == len(self.high):      # even count: the mean of the two tops
            return (-self.low[0] + self.high[0]) / 2
        return -self.low[0]                      # odd count: the low top

    def show(self):
        return sorted((-v for v in self.low), reverse=True), sorted(self.high)


def true_median(values):
    s = sorted(values)
    n = len(s)
    return s[n // 2] if n % 2 else (s[n // 2 - 1] + s[n // 2]) / 2


def main():
    m, seen = RunningMedian(), []
    for fee in (5, 15, 1, 3, 8, 7):
        m.add(fee)
        seen.append(fee)
        low, high = m.show()
        print(f"{fee:2} arrives: low {low} high {high} median {m.median()} true {true_median(seen)}")
    broken, stream = RunningMedian(), (5, 15, 4, 3, 1)
    for fee in stream:
        broken.add_broken(fee)
    print("no rebalance:", broken.show(), "reads", broken.median(), "true", true_median(stream))


if __name__ == "__main__":
    main()
