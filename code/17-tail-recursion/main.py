"""Tail recursion: plain sum, tail-call sum, and the loop a compiler would make.

Python does not reuse frames for tail calls, so both recursive forms
overflow on deep input. The run proves it.
Run: python3 main.py
"""
import inspect


def depth():
    return len(inspect.stack())  # frames on the stack right now


def sum_plain(xs, i=0):
    if i == len(xs):                     # base case: past the end
        return 0
    return xs[i] + sum_plain(xs, i + 1)  # the + waits for the inner call


def sum_tail(xs, i=0, acc=0):
    if i == len(xs):                     # base case: the answer is ready
        return acc
    return sum_tail(xs, i + 1, acc + xs[i])  # the call is the last action


def sum_loop(xs):
    """What tail call optimisation turns sum_tail into: same i, same acc."""
    i, acc = 0, 0                        # the first call's arguments
    while True:
        if i == len(xs):                 # the same base case
            return acc
        i, acc = i + 1, acc + xs[i]      # a jump with new arguments, no push


def sum_plain_traced(xs, i=0, base=None):
    base = depth() if base is None else base
    if i == len(xs):
        print(f"  sum({i}) returns 0 at stack depth {depth() - base + 1}")
        return 0
    rest = sum_plain_traced(xs, i + 1, base)
    print(f"  sum({i}) = {xs[i]} + {rest} = {xs[i] + rest}")
    return xs[i] + rest


def main():
    xs = [4, 7, 2, 9]
    print("plain:", sum_plain(xs), " tail:", sum_tail(xs), " loop:", sum_loop(xs))
    print("plain recursion, unwinding:")
    sum_plain_traced(xs)
    big = [1] * 100_000
    for fn in (sum_plain, sum_tail):
        try:
            fn(big)
        except RecursionError as err:
            print(f"{fn.__name__} on 100,000 items: RecursionError ({err})")
    print("sum_loop on 100,000 items:", sum_loop(big), "with one frame")


if __name__ == "__main__":
    main()
