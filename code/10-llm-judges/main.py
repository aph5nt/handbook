"""LLM judges: a pairwise judge with order swap, a win rate, agreement with people.

Example numbers, not measurements. Standard library only. Run: python3 main.py
"""
import re

KNOWN = {"0x4a21...ee02", "0x77c1...30ff"}   # addresses the store holds
ADDRESS = re.compile(r"0x[0-9a-f]{4}\.\.\.[0-9a-f]{4}")

# Investigators' picks on 50 pairs of summaries (version A against version B).
HUMAN = ["B"] * 32 + ["A"] * 14 + ["tie"] * 4

def win_rate(picks, side="B"):
    """Share of pairs the side wins, and the same counting a tie as half."""
    wins = picks.count(side)
    ties = picks.count("tie")
    return wins / len(picks), (wins + ties / 2) / len(picks)

def agreement(judge_picks, human_picks):
    """Share of pairs where the judge picked what people picked."""
    same = sum(1 for j, h in zip(judge_picks, human_picks) if j == h)
    return same, same / len(human_picks)

def toy_judge(left, right):
    """A biased model judge: it rates longer text higher and favours the first shown."""
    def rate(answer):
        return answer["quality"] + len(answer["text"]) / 20   # length bias
    return "left" if rate(left) + 1 >= rate(right) else "right"   # +1 = position bias

def judge_both_orders(a, b):
    """Judge the pair twice, swapped. Two different verdicts count as a tie."""
    first = a if toy_judge(a, b) == "left" else b
    second = b if toy_judge(b, a) == "left" else a
    return first["name"] if first is second else "tie"

def facts_ok(answer):
    """Code checks every address against the store before any judge sees it."""
    return all(addr in KNOWN for addr in ADDRESS.findall(answer["text"]))

def judge_pair(a, b):
    """Facts in code first, then the swapped judge only on what code cannot check."""
    ok_a, ok_b = facts_ok(a), facts_ok(b)
    if ok_a != ok_b:
        return a["name"] if ok_a else b["name"]
    return judge_both_orders(a, b)

# Tab 1: people, then the model judge, then calibration.
plain, half = win_rate(HUMAN)
print(f"B wins {HUMAN.count('B')}, A wins {HUMAN.count('A')}, ties {HUMAN.count('tie')}")
print(f"win rate of B {plain:.0%}, counting a tie as half {half:.0%}")       # 64%, 68%
llm = list(HUMAN)
for k in range(32, 37):          # the model disagrees on 5 of the pairs people gave A
    llm[k] = "B"
same, share = agreement(llm, HUMAN)
print(f"LLM judge agrees on {same} of 50: {share:.0%}")                       # 45, 90%
llm_new_rubric = list(llm)
for k in range(37, 41):          # after a rubric change, 4 more disagree
    llm_new_rubric[k] = "B"
same, share = agreement(llm_new_rubric, HUMAN)
print(f"after the rubric change: {same} of 50: {share:.0%}")                  # 41, 82%

# Tab 2: the biases and the fixes.
a = {"name": "A", "quality": 5, "text": "Risk high: sent to an exchange."}
b = {"name": "B", "quality": 5, "text": "Risk high: paid an exchange."}
print("A on the left ->", "A" if toy_judge(a, b) == "left" else "B")           # A
print("B on the left ->", "B" if toy_judge(b, a) == "left" else "A")           # B
print("both orders ->", judge_both_orders(a, b))                              # tie

long_a = {"name": "A", "quality": 4,
          "text": "Risk high. Long, confident, cites 0x7a21...c4e9 as the cash-out hub."}
short_b = {"name": "B", "quality": 4, "text": "Risk high: 0x4a21...ee02 is an exchange."}
print("judge alone ->", judge_both_orders(long_a, short_b))                   # A
print("facts check on A:", facts_ok(long_a))                                  # False
print("facts first ->", judge_pair(long_a, short_b))                          # B
