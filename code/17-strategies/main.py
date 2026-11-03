"""Three strategies from the lesson: sliding window, two pointers, greedy coins.

Run: python3 main.py
"""
from collections import deque


class SlidingWindowLimiter:
    def __init__(self, limit, window):
        self.limit, self.window = limit, window
        self.times = deque()                 # request times still inside the window

    def allow(self, now):
        while self.times and self.times[0] <= now - self.window:
            self.times.popleft()             # drop at the back: fell out of the window
        if len(self.times) >= self.limit:    # one compare: count against limit
            return False                     # refused, and not recorded
        self.times.append(now)               # add at the front
        return True


def recount(all_times, now, window):
    """The brute force it replaces: read every timestamp for every request."""
    return sum(1 for t in all_times if now - window < t <= now)


def two_sum_sorted(a, target, log=None):
    left, right = 0, len(a) - 1
    while left < right:
        total = a[left] + a[right]
        if log is not None:
            log.append((a[left], a[right], total))
        if total == target:
            return left, right               # found
        if total > target:
            right -= 1                       # too big: a[right] is too big for anyone left
        else:
            left += 1                        # too small: a[left] is too small for anyone left
    return None                              # pointers met: no pair


def two_sum_seen(a, target):
    """The fix for unsorted data: a hash set of numbers seen so far."""
    seen = set()
    for x in a:
        if target - x in seen:
            return target - x, x
        seen.add(x)
    return None


def greedy_coins(coins, amount):
    took = []
    while amount > 0:
        coin = max(c for c in coins if c <= amount)  # biggest coin that still fits
        took.append(coin)                    # commit, never go back
        amount -= coin
    return took


def best_coins(coins, amount):
    """Dynamic programming: the truth greedy is measured against."""
    best = [[]] + [None] * amount
    for a in range(1, amount + 1):
        options = [best[a - c] + [c] for c in coins if c <= a and best[a - c] is not None]
        best[a] = min(options, key=len)
    return best[amount]


def coin_story(coins, amount):
    g, b = greedy_coins(coins, amount), best_coins(coins, amount)
    verdict = "greedy is best" if len(g) == len(b) else "greedy lost"
    print(f"coins {coins} for {amount}: greedy {g}, best {b}: {verdict}")


def main():
    lim = SlidingWindowLimiter(4, 60)
    for t in (2, 10, 25, 40, 55, 65, 75):
        ok = lim.allow(t)
        print(f"request at {t:2}: {'allowed' if ok else 'refused'}, window ({t - 60}, {t}], inside: {list(lim.times)}")
    print("recount at 75 reads all", 6, "timestamps:", recount([2, 10, 25, 40, 65, 75], 75, 60), "inside")
    log = []
    print("sorted two pointers:", two_sum_sorted([1, 4, 6, 9, 12, 15, 19, 24], 21, log), log)
    log = []
    print("unsorted two pointers:", two_sum_sorted([24, 6, 1, 19, 12, 4, 15, 9], 21, log), log)
    print("unsorted with a hash set:", two_sum_seen([24, 6, 1, 19, 12, 4, 15, 9], 21))
    coin_story([1, 3, 4], 6)
    coin_story([1, 5, 10, 25], 30)


if __name__ == "__main__":
    main()
