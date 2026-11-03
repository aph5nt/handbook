"""Debate: two sides and a judge.

Replays visual/09-debate.html. Standard library only, no network.
The two sides are fake models: fixed arguments per brief. The judge is real
code: it scores only claims that cite facts in the shared evidence.
Run: python3 main.py
"""

EVIDENCE = {"inputs": "about 800 inputs", "equal": "outputs of equal size",
            "sweep": "all outputs swept to one exchange",
            "single": "inputs are single deposits from separate users"}

ARGUMENTS = {  # what each prompt makes the model say: (round, claim, cited fact ids)
    "argue W is a mixer": [(1, "800 inputs, equal outputs: mixing", ["inputs", "equal"]),
                           (2, "no output goes back to a fresh wallet", [])],
    "argue W is not a mixer": [(1, "all outputs swept to one exchange: deposit sweep", ["sweep"]),
                               (2, "inputs are single deposits by separate users", ["single"])],
    "agree with the other side": [(1, "you make a strong point, it is likely a mixer", []),
                                  (2, "I agree", [])],
}


def fake_side(brief):
    """Stands in for one model playing one side."""
    return {"brief": brief, "claims": ARGUMENTS[brief]}


# panel:start
def debate(evidence, brief_a, brief_b):                  # @1 same evidence for both
    a = fake_side(brief_a)                               # @2
    b = fake_side(brief_b)                               # @3 a different prompt or model
    return a, b                                          # @4,5 rounds 1 and 2 inside


def score(side, evidence, fair=True, first=False):
    if not fair:                                         # @11 a biased judge
        words = sum(len(c.split()) for _, c, _ in side["claims"])
        return words + (3 if first else 0)               # @11 longer and first wins
    total = 0
    for rnd, claim, cited in side["claims"]:             # @6
        valid = [f for f in cited if f in evidence]      # @7 uncited claims score 0
        total += len(valid) * (2 if rnd == 2 else 1)     # @6 answering the other side counts double
    return total


def judge(a, b, evidence, fair=True):
    runs = []
    for x, y in [(a, b), (b, a)]:                        # @8 order swapped
        sx = score(x, evidence, fair, first=True)
        sy = score(y, evidence, fair, first=False)
        runs.append(x["brief"] if sx > sy else y["brief"] if sy > sx else None)
    if not fair:                                         # @9
        return runs[0]                                   # @9 one run, no swap
    return runs[0] if runs[0] == runs[1] else "undecided"  # @9


def save(verdict, a, b):
    return {"verdict": verdict, "sides": [a, b]}         # @10 the reasoning stays checkable
# panel:end


def verdict_word(brief):
    return {"argue W is a mixer": "mixer", "argue W is not a mixer": "deposit sweep, not a mixer"}.get(brief, brief)


def main():
    print("evidence:", list(EVIDENCE.values()))
    print("\n-- tab 1: two real sides --")
    a, b = debate(EVIDENCE, "argue W is a mixer", "argue W is not a mixer")
    for side in (a, b):
        for rnd, claim, cited in side["claims"]:
            print(f"  round {rnd}, {side['brief']}: {claim}  cites {cited or 'nothing'}")
    r1 = lambda side: sum(len([f for f in c if f in EVIDENCE]) for r, _, c in side["claims"] if r == 1)
    print(f"  after round 1: A {r1(a)}, B {r1(b)} (1 point per cited fact)")
    print(f"  scores: A {score(a, EVIDENCE)}, B {score(b, EVIDENCE)} (round 2 citations count double)")
    print("  verdict, both orders:", verdict_word(judge(a, b, EVIDENCE)))
    print("  saved:", sorted(save(judge(a, b, EVIDENCE), a, b)))

    print("\n-- tab 2: the same voice twice --")
    a, b = debate(EVIDENCE, "argue W is a mixer", "agree with the other side")
    print("  B says:", b["claims"][0][1])
    words = lambda side: sum(len(c.split()) for _, c, _ in side["claims"])
    print(f"  words: A {words(a)} + 3 for going first, B {words(b)}")
    print(f"  biased judge (length + first): A {score(a, EVIDENCE, False, True)}, "
          f"B {score(b, EVIDENCE, False, False)} -> {verdict_word(judge(a, b, EVIDENCE, fair=False))} (wrong)")
    a, b = debate(EVIDENCE, "argue W is a mixer", "argue W is not a mixer")
    print("  fix: different briefs, swapped order, cited evidence only ->", verdict_word(judge(a, b, EVIDENCE)))


if __name__ == "__main__":
    main()
