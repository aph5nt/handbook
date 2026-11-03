"""LLM-as-judge: a model grades against a rubric.

Replays visual/09-llm-as-judge.html. Standard library only, no network.
Hard checks are real code. The judge model is a fake: rules over the reference
answer stand in for a model's yes-or-no replies, and biased fakes show the
failures. Calibration labels are an example set. Run: python3 main.py
"""
import re

FUNDER = "0x18aa4b2c3d4e5f60718293a4b5c6d7e8f90acb24"
REFERENCE_EXCHANGES = ["Exchange A", "Exchange B"]
ANSWER_A = f"Funded by {FUNDER}. The money reached Exchange A, tokens worth about 900 USD."
ANSWER_B = f"Funded by {FUNDER}. The money reached Exchange A and Exchange B, tokens worth about 900 USD."
LONG_WRONG = (f"Funded by {FUNDER}. After a careful and thorough review of every path, I am very confident "
              "that the money reached Exchange C, and this is clearly the full story of this wallet.")
SHORT_RIGHT = ANSWER_B
COLUMN_NAMES = ["is_contract", "delegated_to", "nonce"]
RUBRIC = ["every exchange named", "amounts in plain words", "no unsupported claims"]


def fake_judge_question(answer, q):
    """Stands in for a judge model answering one yes-or-no rubric question."""
    if q == RUBRIC[0]:
        missing = [e for e in REFERENCE_EXCHANGES if e not in answer]
        return not missing, f"missing {missing[0]}" if missing else "all named"
    if q == RUBRIC[1]:
        return not any(c in answer for c in COLUMN_NAMES), "no column names"
    named = re.findall(r"Exchange [A-Z]", answer)
    extra = [e for e in named if e not in REFERENCE_EXCHANGES]
    return not extra, f"{extra[0]} not in the rows" if extra else "none"


def rubric_score(answer):
    return sum(fake_judge_question(answer, q)[0] for q in RUBRIC)


def fake_pick(first, second):
    """Fair fake: the better rubric score wins, the order does not matter."""
    return first if rubric_score(first) > rubric_score(second) else second


# panel:start
def hard_checks(answer):
    addresses = re.findall(r"0x[0-9a-fA-F]+", answer)
    return all(len(a) == 42 for a in addresses) and "MATCH (" not in answer  # @1 code, exact and free


def grade(answer):
    if not hard_checks(answer):                          # @2 no judge call
        return False, ["failed the hard checks"]         # @2
    # the judge is a different model from the writer       # @3
    verdicts = {}
    for q in RUBRIC:                                     # @4 yes-or-no questions
        verdicts[q] = fake_judge_question(answer, q)     # @5 a verdict with a reason
    return all(v[0] for v in verdicts.values()), verdicts  # @6


def pairwise(a, b, pick=fake_pick):
    first = pick(a, b)                                   # @7 A shown first
    second = pick(b, a)                                  # @7 B shown first
    return first if first == second else None            # @8 keep only if both agree


def calibrated(judge_labels, human_labels):
    agree = sum(j == h for j, h in zip(judge_labels, human_labels))  # @9
    return agree, agree / len(human_labels) >= 0.8       # @10 trust at 80 % or more
# panel:end


def main():
    print("-- tab 1: a verdict with reasons --")
    print("hard checks on A:", hard_checks(ANSWER_A))
    ok, verdicts = grade(ANSWER_A)
    for q, (good, why) in verdicts.items():
        print(f"  {q}: {'yes' if good else 'no'} ({why})")
    print("pass:", ok)
    print(f"rubric: A {rubric_score(ANSWER_A)} of 3, B {rubric_score(ANSWER_B)} of 3")
    print("pairwise: B wins both orders ->", pairwise(ANSWER_A, ANSWER_B) == ANSWER_B)
    judge = [1, 1, 0, 1, 0, 1, 1, 0, 1, 1]
    human = [1, 1, 0, 1, 0, 1, 0, 0, 1, 1]
    agree, trust = calibrated(judge, human)
    print(f"calibration: judge agrees with people on {agree} of {len(human)}, trust (80 % or more): {trust}")

    print("\n-- tab 2: the judge's biases --")
    print("self-graded, no rubric: 9 out of 10 (a fake model that likes its own style)")
    by_length = lambda x, y: x if len(x) > len(y) else y
    print("length bias picks the wrong answer:", by_length(LONG_WRONG, SHORT_RIGHT) == LONG_WRONG)
    print("rubric on the long wrong answer:", rubric_score(LONG_WRONG), "of 3; on the short right one:", rubric_score(SHORT_RIGHT), "of 3")
    first_wins = lambda x, y: x
    print("position bias, A first:", "A" if first_wins(ANSWER_A, ANSWER_B) == ANSWER_A else "B",
          "| B first:", "A" if first_wins(ANSWER_B, ANSWER_A) == ANSWER_A else "B")
    print("both orders disagree, pairwise keeps:", pairwise(ANSWER_A, ANSWER_B, first_wins))
    biased = [0, 1, 1, 1, 1, 1, 0, 0, 0, 1]
    agree, trust = calibrated(biased, human)
    print(f"calibration of the biased judge: {agree} of {len(human)}, trust: {trust}")
    print("fix: pairwise with the fair judge ->", "B" if pairwise(ANSWER_A, ANSWER_B) == ANSWER_B else "A")


if __name__ == "__main__":
    main()
