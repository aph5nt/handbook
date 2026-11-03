"""Fibonacci three ways: naive recursion, memoization, bottom-up loop.

Each version counts its calls or loop turns, so the lesson's numbers are proven.
Run: python3 main.py
"""

calls = 0


def fib_naive(n):
    global calls
    calls += 1
    if n < 2:                                  # base cases: fib(0) = 0, fib(1) = 1
        return n
    return fib_naive(n - 1) + fib_naive(n - 2)  # two calls, every time


cache = {}
hits = 0


def fib_memo(n):
    global calls, hits
    calls += 1
    if n in cache:                             # asked before: a lookup, no subtree
        hits += 1
        return cache[n]
    if n < 2:
        answer = n
    else:
        answer = fib_memo(n - 1) + fib_memo(n - 2)
    cache[n] = answer                          # store it before returning
    return answer                              # the cache never forgets a key


def fib_iter(n):
    a, b = 0, 1                                # fib(0) and fib(1)
    for turn in range(n):                      # n turns, no calls
        a, b = b, a + b                        # slide the two-answer window up
    return a


def naive_calls(n):
    """Calls fib_naive(n) makes, without running it: 1 + calls(n-1) + calls(n-2)."""
    c = [1, 1]
    for k in range(2, n + 1):
        c.append(1 + c[k - 1] + c[k - 2])
    return c[n]


def main():
    global calls, hits
    calls = 0
    print("naive fib(6) =", fib_naive(6), "in", calls, "calls")
    calls = 0
    fib_naive(25)
    print("naive fib(25) measured", f"{calls:,}", "calls, formula", f"{naive_calls(25):,}")
    print("naive fib(40) would make", f"{naive_calls(40):,}", "calls")
    calls, hits = 0, 0
    print("memo fib(6) =", fib_memo(6), "in", calls, "calls:", calls - hits, "computed,", hits, "from the cache")
    cache.clear()
    calls, hits = 0, 0
    fib_memo(40)
    print("memo fib(40):", calls, "calls, cache now holds", len(cache), "keys and keeps them")
    a, b = 0, 1
    for turn in range(1, 7):
        a, b = b, a + b
        print(f"iter turn {turn}: a = {a}, b = {b}")
    print("iter fib(6) =", fib_iter(6), " fib(40) =", fib_iter(40), "in 40 turns")


if __name__ == "__main__":
    main()
