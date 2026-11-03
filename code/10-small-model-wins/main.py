"""When a small model wins: replay the lesson of the same name.

A skill router on 100 golden questions, an escalation path, and the total
cost of "small first" for fund traces. Costs are example units, not prices.
Standard library only, no network. Run: python3 main.py
"""

THRESHOLD = 0.95          # the small model must pass this share of the golden set
SMALL, BIG = 2, 10        # example cost units per call
SKILLS = ["dex", "virtuals", "address-risk", "bridge", "mixer", "exchange",
          "pricing", "launchpad", "nft", "governance", "staking", "lending"]

# The golden set: 100 questions with the right skill. The small model's answers
# miss on cases 38 and 82 (index 37 and 81), as in the lesson's grid.
golden = [SKILLS[k % 12] for k in range(100)]
small_answers = [s if k not in (37, 81) else "unsure" for k, s in enumerate(golden)]


def run_golden_set(answers):
    return sum(a == g for a, g in zip(answers, golden))


def answer(k):
    """Small first, with a check and a way up."""
    a = small_answers[k]
    if a not in SKILLS:                 # the check: unsure or not one of the 12 labels
        return golden[k], "escalated"   # the big model answers it, slower, not wrong
    return a, "small"


def small_first_cost(escalation_rate):
    return SMALL + escalation_rate * BIG


print("Tab 1: routing, a narrow task")
passed = run_golden_set(small_answers)
print(f"golden set: small model passed {passed} of 100")
print("use small for routing" if passed / 100 >= THRESHOLD else "use big for routing")
routes = [answer(k)[1] for k in range(100)]
print(f"with escalation: {routes.count('small')} answered small, {routes.count('escalated')} escalated, "
      f"{sum(answer(k)[0] == golden[k] for k in range(100))} of 100 right")

print()
print("Tab 2: fund traces, an open task")
print(f"big model alone: {BIG} units per trace")
for rate in (0.5, 0.9):
    c = small_first_cost(rate)
    verdict = "more than big alone" if c >= BIG else "less than big alone, but slower when escalated"
    print(f"small first, {rate:.0%} escalate: {SMALL} + {rate} x {BIG} = {c:g} units, {verdict}")
print(f"break-even escalation rate: {(BIG - SMALL) / BIG:.0%}")
