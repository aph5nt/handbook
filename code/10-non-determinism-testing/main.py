"""One run proves nothing: replay the lesson of the same name.

The model is a stand-in that replays recorded run results (1 = it kept the
address in full, 0 = it shortened it), so the numbers match the lesson.
The check, the pass rate, the thresholds and the with/without comparison
are the real part. Standard library only. Run: python3 main.py
"""
import re

FULL = "0x9f3c5a2e8b1d4f6a7c0e9b3d2a1f8e7c6b5d0071"    # 42 characters
SHORT = "0x9f3c...0071"                                # shortened: 13

R10 = [1, 1, 0, 1, 1, 0, 1, 1, 0, 1]            # ten runs of one prompt
NOT = [[0, 1, 0], [0, 0, 1], [1, 0, 0], [1, 1, 0], [0, 0, 1]]   # 5 cases x 3, no skill
WITH = [[1, 1, 1], [1, 1, 1], [1, 1, 1], [1, 0, 1], [1, 1, 1]]  # same, with the skill


def model(kept):                               # one sampled answer
    return f"The wallet was funded by {FULL if kept else SHORT}."


def check(answer):                             # hard check: every address in full
    return all(len(a) == 42 for a in re.findall(r"0x[0-9a-fA-F]+(?:\.\.\.[0-9a-fA-F]+)?", answer))


def pass_rate(runs):
    passed = sum(check(model(r)) for r in runs)
    return passed, len(runs)


def verdict(passed, total, line):
    return "meets" if passed / total >= line else "below"


print("Tab 1: one run versus ten")
p, t = pass_rate(R10[:1])
print(f"  run 1: {'pass' if p else 'fail'} -> one run says 'ship it'")
p, t = pass_rate(R10)
print(f"  ten runs: {p} of {t} pass = {p / t:.0%}")
print(f"  soft line 90%: {verdict(p, t, 0.90)}, hard line 100%: {verdict(p, t, 1.0)}")
p, t = pass_rate([1] * 10)                     # rule moved into the tool description
print(f"  after the fix: {p} of {t} = {p / t:.0%}, hard line: {verdict(p, t, 1.0)}")

print()
print("Tab 2: with versus without the skill")
pn, tn = pass_rate([r for case in NOT for r in case])
pw, tw = pass_rate([r for case in WITH for r in case])
print(f"  without: {pn} of {tn} = {pn / tn:.0%}")
print(f"  with:    {pw} of {tw} = {pw / tw:.0%}")
delta = round(100 * pw / tw) - round(100 * pn / tn)
print(f"  delta: +{delta} points")
for k, runs in enumerate(WITH, 1):
    if len(set(runs)) > 1:
        print(f"  case {k} flips with the skill: {runs}")
a, b = check(model(NOT[3][0])), check(model(WITH[3][1]))
print(f"  one run each, case 4: without run 1 = {'pass' if a else 'fail'}, "
      f"with run 2 = {'pass' if b else 'fail'} -> says the skill hurts. Wrong.")
