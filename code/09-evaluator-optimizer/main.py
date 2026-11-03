"""Evaluator-optimizer: write, grade, fix, repeat.

Replays visual/09-evaluator-optimizer.html. The producer is a fake: with
specific feedback it fixes only the named item; with vague feedback it
rewrites everything and gets longer and worse. The evaluator is code over
the rubric. Standard library only, no network. Run: python3 main.py
"""
import re

FUNDER_A = "0x18aa4b2c3d4e5f60718293a4b5c6d7e8f90acb24"
FUNDER_B = "0x77c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a730ff"
DRAFT_1 = f"Funder A {FUNDER_A} sent tokens worth about 900 USD to Exchange A."


def fake_producer(task, draft=None, feedback=None, round_no=1):
    if draft is None:
        return DRAFT_1
    if feedback and feedback.startswith("failed: every exchange named"):
        return draft + f" Funder B {FUNDER_B} sent tokens worth about 400 USD to Exchange B."
    # vague feedback: rewrite all of it, longer each round
    text = draft + " In summary, and to be clear, the flows are notable."
    if round_no == 3:
        text = text.replace(FUNDER_A, FUNDER_A[:6] + "..." + FUNDER_A[-4:])
    if round_no >= 5:
        text = text.replace("about 900 USD", "0x38d7ea4c68000 wei")
    return text


# panel:start
RUBRIC = {  # @1
    "addresses in full": lambda d: all(len(a) == 42 for a in re.findall(r"0x[0-9a-fA-F]+(?:\.\.\.[0-9a-fA-F]+)?", d)),  # @1
    "every exchange named": lambda d: "Exchange A" in d and "Exchange B" in d,  # @1
    "amounts in plain words": lambda d: "wei" not in d,  # @1
}


def evaluate(draft, specific=True):
    failed = [name for name, ok in RUBRIC.items() if not ok(draft)]  # @4
    if not failed:
        return failed, ""
    if specific:  # @7 name the failed item and where
        return failed, f"failed: {failed[0]} (the exchange reached through Funder B is missing)"  # @7
    return failed, "could be better, improve clarity"


def optimise(task, cap=3, specific=True, keep_best=True, log=print):
    draft = fake_producer(task)  # @2
    best, best_score = draft, -1
    for round_no in range(1, cap + 1):  # @3 the round cap
        failed, feedback = evaluate(draft, specific)  # @4
        score = len(RUBRIC) - len(failed)  # @5
        if score > best_score:  # @5 keep the best so far
            best, best_score = draft, score  # @5
        log(f"round {round_no}: {score} of {len(RUBRIC)} items pass, {len(draft)} characters")
        if not failed:  # @6
            return draft  # @6 every item passes
        if round_no < cap:  # @3 no new draft after the last round
            draft = fake_producer(task, draft, feedback, round_no + 1)  # @8 fix that item, keep the rest
    return best if keep_best else draft  # @9 the best, not the last
# panel:end


def main():
    task = "Where did the money of wallet 0x9f2c4e7a1b3d5f6e8a0c2b4d6f8e0a1c3b5d0071 go?"
    print("-- tab 1: specific feedback, round cap 3 --")
    print("first feedback:", evaluate(DRAFT_1)[1])
    out = optimise(task)
    print("output passes every item:", not evaluate(out)[0])
    print("-- tab 2: vague feedback, no cap, the last version ships --")
    out = optimise(task, cap=6, specific=False, keep_best=False)
    print(f"shipped: the round 6 draft, {len(out)} characters,", len(RUBRIC) - len(evaluate(out)[0]), "of 3 items pass")
    print("-- the fix: specific feedback, cap 3, best kept --")
    out = optimise(task, log=lambda s: None)
    print("kept:", len(RUBRIC) - len(evaluate(out)[0]), "of 3 items pass")


if __name__ == "__main__":
    main()
