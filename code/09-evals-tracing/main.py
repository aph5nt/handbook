"""Evals, skills and tracing: the eval loop for one skill.

Replays tab 1 of visual/09-evals-tracing.html, then runs tracing.py for tab 2.
Standard library only, no network. The agent and the judge are fakes: fixed
answers per run, so every run prints the same numbers as the lesson.
Run: python3 main.py
"""
import re

import tracing

FULL = "0x9f2c4e7a1b3d5f6e8a0c2b4d6f8e0a1c3b5d0071"
EXCHANGES = ["Exchange A", "Exchange B"]
SKILL = {"name": "address-rules", "rule": "write every address in full, name every exchange"}

# Answers of the three runs without the skill. With the skill, every run is right.
WITHOUT = [
    f"{FULL} sent money to Exchange A and Exchange B.",                    # right
    f"{FULL[:6]}...{FULL[-4:]} sent money to Exchange A.",                 # short address
    f"USE topology MATCH (a) RETURN a. {FULL} sent money to Exchange A and Exchange B.",  # query text
]


def fake_agent(question, with_skill, run):
    if with_skill:
        return f"{FULL} sent money to Exchange A and Exchange B."
    return WITHOUT[run]


def fake_judge(answer, question):
    """Answers one rubric question with a rule: is every exchange named?"""
    return all(e in answer for e in EXCHANGES)


# panel:start
def load_skill():
    return SKILL  # @1 description + rules, a file in git


def load_cases():
    # each wrong answer that reached a user becomes a case  # @2
    return [{"question": "Who did this wallet send money to?",  # @2
             "reason": "an address was shortened in a table"}]  # @2


def hard_check(answer):
    addresses = re.findall(r"0x[0-9a-fA-F]+(?:\.\.\.[0-9a-fA-F]+)?", answer)  # @5
    full = all(len(a) == 42 for a in addresses)  # @5 every address 42 characters
    no_query = "MATCH" not in answer  # @5 no query text printed
    return full and no_query


def run_eval(cases, runs=3):
    counts = {}
    for case in cases:
        for with_skill in (True, False):  # @3
            hard = soft = both = 0
            for run in range(runs):  # @4
                answer = fake_agent(case["question"], with_skill, run)  # @4
                h = hard_check(answer)  # @5
                s = fake_judge(answer, case["question"])  # @6 only what code cannot check
                hard, soft, both = hard + h, soft + s, both + (h and s)  # @7
            counts[with_skill] = (hard, soft, both)
    return counts


def decide(counts):
    delta = counts[True][2] - counts[False][2]  # @8 with minus without
    return ("merge" if delta > 0 else "fix the skill text"), delta  # @9
# panel:end


def main():
    skill = load_skill()
    print("skill:", skill["name"], "-", skill["rule"])
    cases = load_cases()
    print("case:", cases[0]["reason"], f"({FULL[:6]}...{FULL[-4:]})")
    counts = run_eval(cases)
    w, wo = counts[True], counts[False]
    print(f"hard checks: with {w[0]} of 3, without {wo[0]} of 3")
    print(f"judge: with {w[1]} of 3, without {wo[1]} of 3")
    verdict, delta = decide(counts)
    print(f"pass both: with {w[2]} of 3, without {wo[2]} of 3, delta +{delta} -> {verdict}")
    print()
    tracing.main()


if __name__ == "__main__":
    main()
