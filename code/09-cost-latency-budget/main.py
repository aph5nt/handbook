"""Cost and latency budgets.

Replays visual/09-cost-latency-budget.html. Every token and second is an
example number from a table, added up by the code. Time is simulated, so the
program returns at once. Run: python3 main.py
"""


class OverBudget(Exception):
    pass


# panel:start
class Budget:
    def __init__(self, tokens=10_000, steps=6, seconds=10.0, enforce=True):  # @1
        self.max = (tokens, steps, seconds)            # @1
        self.tokens, self.steps, self.seconds = 0, 0, 0.0
        self.enforce = enforce

    def spend(self, name, tokens, seconds):
        self.tokens += tokens                          # @7 what the trace records
        self.seconds += seconds                        # @7
        self.steps += 1                                # @7
        print(f"  {name}: +{tokens:,} tokens, +{seconds:g} s -> {self.tokens * 100 // self.max[0]} % of "
              f"{self.max[0]:,} tokens, {self.seconds:g} of {self.max[2]:g} s, step {self.steps}")  # @7
        if self.enforce and (self.tokens > self.max[0] or self.steps > self.max[1]
                             or self.seconds > self.max[2]):  # @6
            raise OverBudget(name)                     # @6 return the best partial answer


def parallel_wait(times):                              # @5
    return max(times)                                  # @5 the wait is the slowest


def good_request(b):
    b.spend("classify with the small model", 500, 0.5)              # @2
    b.spend("system prompt + tools from cache", 1500, 1.5)          # @3
    b.spend("tool choice, output capped", 1000, 1.0)                # @4
    b.spend("3 lookups in parallel", 2500, parallel_wait([3, 3, 2]))  # @5
    b.spend("final answer, output capped", 1500, 2.0)               # @4
# panel:end


def bad_request(b):
    b.spend("classify with the large model", 1000, 1.0)
    b.spend("system prompt paid in full", 3000, 3.0)
    b.spend("3 lookups in a row", 2500, sum([3, 3, 2]))  # the same lookups, one by one
    tokens, seconds = 33_500 / 25, 58 / 25
    for _ in range(25):
        b.tokens += tokens
        b.seconds += seconds
        b.steps += 1
    print(f"  25 retries at {tokens:,.0f} tokens and {seconds:.1f} s each: {int(b.tokens * 100 / b.max[0])} % of tokens, {b.seconds:g} s, step {b.steps}, no answer")


def main():
    print("-- tab 1: five levers, one budget --")
    b = Budget()
    good_request(b)
    print(f"  done: inside every cap, {b.steps} steps")

    print("\n-- tab 2: no budget --")
    bad_request(Budget(enforce=False))

    print("\n-- the fix: the same bad request with the cap on --")
    b = Budget()
    try:
        b.spend("classify with the large model", 1000, 1.0)
        b.spend("system prompt paid in full", 3000, 3.0)
        b.spend("3 lookups in a row", 2500, sum([3, 3, 2]))
    except OverBudget as e:
        print(f"  over budget at '{e}': stop and return the best partial answer")


if __name__ == "__main__":
    main()
