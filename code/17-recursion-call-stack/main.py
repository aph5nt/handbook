"""Recursion and the call stack: fact(4) frame by frame, then the missing base case.

Run: python3 main.py
"""
import sys

depth = 0      # frames of fact on the stack right now
deepest = 0    # the most frames seen at once
trace = []


def fact(n):
    global depth, deepest
    depth += 1                         # a call pushes a frame holding its own n
    deepest = max(deepest, depth)
    trace.append(f"{'  ' * (depth - 1)}push fact({n})")
    if n == 0:                         # the base case: the floor
        result = 1
    else:
        result = n * fact(n - 1)       # this frame waits here for the answer
    trace.append(f"{'  ' * (depth - 1)}pop  fact({n}) returns {result}")
    depth -= 1                         # return pops the frame
    return result


def fact_no_base(n):
    global depth, deepest
    depth += 1
    deepest = max(deepest, depth)
    # base case forgotten: nothing says when to stop
    return n * fact_no_base(n - 1)


def fact_loop(n):
    """The fix for deep input: a loop keeps one frame, whatever n is."""
    result = 1
    while n > 0:
        result *= n
        n -= 1
    return result


def main():
    global depth, deepest
    print("fact(4) =", fact(4), " deepest stack:", deepest, "frames")
    print("\n".join(trace))
    depth = deepest = 0
    try:
        fact_no_base(4)
    except RecursionError as err:
        print(f"no base case: {type(err).__name__}: {err}")
        print(f"  frames of fact_no_base when it stopped: {deepest}"
              f" (the limit is {sys.getrecursionlimit()} frames in all, main and the rest included)")
        print(f"  the last call was fact_no_base({4 - deepest + 1})")
    depth = deepest = 0
    try:
        fact(100_000)  # correct code, deep input
    except RecursionError:
        print("correct fact(100000) also hits RecursionError after", deepest, "frames")
    print("fact_loop(100000) finishes:", fact_loop(100_000).bit_length(), "bits long, using one frame")


if __name__ == "__main__":
    main()
