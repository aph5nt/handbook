"""Agent evals: ten runs, not one.

Replays visual/09-agent-evals.html. Standard library only, no network.
The agent is a fake: a fixed table says which runs of each variant drop an
exchange or shorten an address, so every run of this program prints the same
numbers as the lesson. Run: python3 main.py
"""
import re

TASK = "Who funded wallet 0x9f2c4e7a1b3d5f6e8a0c2b4d6f8e0a1c3b5d0071? Name every exchange."
EXCHANGES = ["Exchange A", "Exchange B"]
FUNDER = "0x18aa4b2c3d4e5f60718293a4b5c6d7e8f90acb24"

# p = this run gets it right, s = shortens an address, x = drops an exchange
BEHAVIOUR = {
    "no skill": "psppxsxpsx",   # 4 of 10 right
    "skill v1": "ppxpppppsp",   # 8 of 10 right
    "skill v2": "psxpxppspx",   # 5 of 10 right
}


def fake_agent(task, variant, run):
    """Stands in for one full agent run. Returns the answer text."""
    kind = BEHAVIOUR[variant][run]
    funder = FUNDER[:6] + "..." + FUNDER[-4:] if kind == "s" else FUNDER
    named = EXCHANGES[:1] if kind == "x" else EXCHANGES
    return f"Funded by {funder}. Money reached {', '.join(named)}."


# panel:start
RUNS = 10                                         # @1 one run proves nothing


def check(answer):                                # @2
    addresses = re.findall(r"0x[0-9a-fA-F]+(?:\.\.\.[0-9a-fA-F]+)?", answer)
    full = all(len(a) == 42 for a in addresses)   # @2 every address in full
    named = all(e in answer for e in EXCHANGES)   # @2 every exchange named
    return full and named


def pass_rate(variant, runs=RUNS):                # @3
    passed = 0
    for run in range(runs):                       # @3
        if check(fake_agent(TASK, variant, run)):  # @4
            passed += 1                           # @4
    return passed


def gate(rates, live, candidate):
    delta = rates[candidate] - rates["no skill"]  # @5 ablation: with minus without
    if rates[candidate] < rates[live]:            # @6
        return f"blocked: {candidate} {rates[candidate]} of {RUNS} < {live} {rates[live]} of {RUNS}"  # @6
    if delta <= 0:                                # @7
        return "blocked: the skill adds nothing"  # @7
    return f"merge: delta +{delta} in {RUNS}"     # @8
# panel:end


def main():
    print("task:", TASK)
    print("\n-- tab 1: ten runs, with and without --")
    rates = {}
    for v in ["no skill", "skill v1"]:
        rates[v] = pass_rate(v)
        print(f"{v}: {rates[v]} of {RUNS} pass")
        for run in range(RUNS):
            ans = fake_agent(TASK, v, run)
            if not check(ans):
                short = [a for a in re.findall(r"0x[0-9a-fA-F]+(?:\.\.\.[0-9a-fA-F]+)?", ans) if len(a) != 42]
                why = f"shortens to {short[0]} ({len(short[0])} characters)" if short else "names only one exchange"
                print(f"  run {run + 1} fails: {why}")
    rates["skill v0"] = 0  # nothing was live before v1
    print("gate v1:", gate(rates, "skill v0", "skill v1"))
    rates["skill v2"] = pass_rate("skill v2")
    print(f"skill v2: {rates['skill v2']} of {RUNS} pass")
    print("gate v2:", gate(rates, "skill v1", "skill v2"))

    print("\n-- tab 2: one lucky run --")
    one = pass_rate("skill v2", runs=1)
    print(f"skill v2, runs=1: {one} of 1 pass -> looks green, it ships")
    print(f"skill v2, real rate: {pass_rate('skill v2')} of {RUNS}")


if __name__ == "__main__":
    main()
