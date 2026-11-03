"""Golden set and ablation: prove the skill does its job.

Replays visual/09-golden-set-ablation.html. Standard library only, no network.
The model is a fake: a fixed table of which runs shorten an address, and a
memorised skill that only works on the exact questions it copied. The checks,
the delta and the held-out score are real code. Run: python3 main.py
"""
import re
from datetime import date

ADDRESS = "0x9f2c4e7a1b3d5f6e8a0c2b4d6f8e0a1c3b5d0071"
SHORT = ADDRESS[:4] + "…" + ADDRESS[-4:]
CASE = {"question": "Who funded this wallet?", "rule": "every address is 42 characters", "added": date(2026, 10, 1)}
GOLDEN = [f"golden question {n}" for n in range(1, 21)]
HELD_OUT = ["table of senders for this wallet", "list the receivers", "compare two wallets",
            "who funded it last week", "name the biggest sender"]
RUN_TABLE = {"with": "ppp", "without": "sps"}   # p = full address, s = shortened


def fake_model(question, skill, run):
    """Stands in for one agent run."""
    if skill == "memorised":
        return ADDRESS if question in GOLDEN else SHORT
    return ADDRESS if RUN_TABLE[skill][run] == "p" else SHORT


def full_addresses(answer):
    return all(len(a) == 42 for a in re.findall(r"0x[0-9a-fA-F…]+", answer))


# panel:start
RUNS = 3


def add_case(golden, question, rule, day):                # @1 a real failure, in git
    golden.append({"question": question, "rule": rule, "added": day})  # @1


def passes(question, skill, runs=RUNS):
    return sum(full_addresses(fake_model(question, skill, r)) for r in range(runs))


def ablate(case):                                         # @2
    with_skill = passes(case["question"], "with")         # @3
    without = passes(case["question"], "without")         # @4 ablation: remove the skill
    delta = with_skill - without                          # @5
    return with_skill, without, delta, "merge" if delta > 0 else "fix the skill"  # @6


def score(questions, skill):
    return sum(full_addresses(fake_model(q, skill, 0)) for q in questions)


def report(golden, held_out, skill):                      # @7 held_out is never shown to the author
    return score(golden, skill), score(held_out, skill)   # @8 the held-out one is honest


def needs_review(case, today, max_age_days=30):           # @9 facts change
    return (today - case["added"]).days > max_age_days    # @9 a stale reference fails the right answer
# panel:end


def main():
    print("-- tab 1: a failure becomes a case --")
    print("failure:", SHORT, f"({len(SHORT)} characters)")
    golden = []
    add_case(golden, CASE["question"], CASE["rule"], CASE["added"])
    print("case:", golden[0]["question"], "| rule:", golden[0]["rule"])
    w, wo, d, verdict = ablate(golden[0])
    print(f"with the skill: {w} of {RUNS} pass")
    print(f"without the skill: {wo} of {RUNS} pass")
    for r in range(RUNS):
        print(f"  run {r + 1} without: {'full' if full_addresses(fake_model(CASE['question'], 'without', r)) else 'shortened'}")
    print(f"delta: +{d} of {RUNS}: {verdict}")

    print("\n-- tab 2: a skill that memorised its cases --")
    g, h = report(GOLDEN, HELD_OUT, "memorised")
    print(f"golden set: {g} of {len(GOLDEN)}")
    print(f"new question '{HELD_OUT[0]}':", fake_model(HELD_OUT[0], "memorised", 0))
    print(f"held-out: {h} of {len(HELD_OUT)} (reported too)")
    today = date(2026, 11, 2)
    print(f"case added {CASE['added']}, on {today} it is {(today - CASE['added']).days} days old, over 30:",
          "needs review", needs_review(CASE, today))


if __name__ == "__main__":
    main()
