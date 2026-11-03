"""pass@k: replay the lesson "pass@k: at least one of k tries passes".

Three sampled answers to one task are checked by a test. Then pass@k is
computed two ways: the independent-tries formula and the standard unbiased
estimator from n samples with c passing. Standard library only.
Run: python3 main.py
"""
from math import comb

# Toy data: transfers out of one wallet, (receiver, amount).
TRANSFERS = [("A", 5), ("B", 9), ("C", 1), ("D", 7), ("A", 3), ("E", 2)]

def expected_top(n):
    totals = {}
    for who, amount in TRANSFERS:
        totals[who] = totals.get(who, 0) + amount
    return sorted(totals.items(), key=lambda kv: -kv[1])[:n]

# Three sampled answers, written as code the test can run.
def sample_1(n):   # forgot to sort
    totals = {}
    for who, amount in TRANSFERS:
        totals[who] = totals.get(who, 0) + amount
    return list(totals.items())[:n]

def sample_2(n):   # correct
    return expected_top(n)

def sample_3(n):   # wrong join: counts each transfer, not the sum
    counts = {}
    for who, _ in TRANSFERS:
        counts[who] = counts.get(who, 0) + 1
    return sorted(counts.items(), key=lambda kv: -kv[1])[:n]

def test(answer):
    """The checker: the answer must equal the true top three."""
    return answer(3) == expected_top(3)

def solved_at_k(samples):
    """A task is solved at k if at least one of its k samples passes."""
    return any(test(s) for s in samples)

def pass_at_k_independent(p, k):
    """If one try passes with probability p, and tries are independent."""
    return 1 - (1 - p) ** k

def pass_at_k_estimate(n, c, k):
    """Unbiased estimator from n samples of which c passed: 1 - C(n-c, k) / C(n, k)."""
    if n - c < k:
        return 1.0
    return 1 - comb(n - c, k) / comb(n, k)

print("Tab 1: one task, three samples")
for name, s in [("sample 1", sample_1), ("sample 2", sample_2), ("sample 3", sample_3)]:
    print(f"{name}: {'passes' if test(s) else 'fails'}")
print("solved at k = 3:", solved_at_k([sample_1, sample_2, sample_3]))
for k in (1, 3, 10):
    print(f"independent tries, p = 0.4: pass@{k} = {pass_at_k_independent(0.4, k):.0%}")
print(f"estimate from 10 samples, 4 passing: pass@1 = {pass_at_k_estimate(10, 4, 1):.0%}, "
      f"pass@3 = {pass_at_k_estimate(10, 4, 3):.0%}")
per_task = [pass_at_k_estimate(10, c, 3) for c in (4, 2, 6)]   # the lesson's three tasks
print("pass@3 per task:", ", ".join(f"{v:.0%}" for v in per_task),
      f"-> pass@3 over the set = {sum(per_task) / len(per_task):.0%}")
print()
print("Tab 2: one try, no test in production -> the honest number is pass@1 = 40%")
