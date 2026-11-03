"""Few-shot and zero-shot: replay the lesson "Few-shot: the model copies everything in the examples".

The program builds the prompt, checks the shape of each answer, and looks for
an accident the examples share. The model's answers are recorded sample
outputs written into this file (no model is called). Standard library only.
Run: python3 main.py
"""
RULE = "Summarise the wallet risk: one verdict line, then three evidence bullets."
MARKERS = ["exchange", "bridge", "mixer"]   # counterparties a summary may name
TOKENS_PER_EXAMPLE = 300           # example size, in tokens
EXAMPLE_BUDGET = 1_500             # most tokens we spend on examples per call


def build_prompt(examples, wallet):
    parts = [RULE]
    for ex_in, ex_out in examples:
        parts += ["Input: " + ex_in, "Output: " + ex_out]
    parts.append("Real input starts here: " + wallet)    # the boundary
    return "\n".join(parts)


def good_shape(answer):
    """Verdict line first, then exactly three bullets (or a single 'Not found')."""
    lines = answer.strip().split("\n")
    if lines == ["Not found"]:
        return True
    return lines[0].startswith("Verdict:") and len(lines) == 4 and all(l.startswith("- ") for l in lines[1:])


def shared_accident(examples):
    """Counterparty words that every example output names: the model will copy them."""
    return [m for m in MARKERS if all(m in out.lower() for _, out in examples)]


GOOD = "Verdict: medium risk\n- funded by two wallets\n- 14 swaps in a day\n- funds reached an exchange"
EXCHANGE_EXAMPLES = [("wallet A", GOOD), ("wallet B", GOOD.replace("medium", "low")),
                     ("wallet C", GOOD.replace("14 swaps", "3 swaps"))]
VARIED_EXAMPLES = [("easy wallet", GOOD),
                   ("hard wallet", "Verdict: high risk\n- funded through a bridge\n- 2 hops from a mixer\n- 40 swaps in an hour"),
                   ("empty wallet", "Not found")]

print("Tab 1: the shape")
zero_shot = ["The wallet looks medium risk because it was funded by two wallets and swapped often.",
             "- funded by two wallets\n- 14 swaps\n- an exchange",
             "| funder | swaps |\n| two | 14 |"]
print("zero-shot, shape ok:", sum(good_shape(a) for a in zero_shot), "of 3")
prompt = build_prompt(EXCHANGE_EXAMPLES, "0x9f3c8a10b2c4d5e6f70892a3b4c5d6e1d27b0071")
print("few-shot prompt ends with:", prompt.split("\n")[-1][:40] + "...")
few_shot = [GOOD, GOOD.replace("medium", "high"), GOOD.replace("14", "9")]
print("few-shot, shape ok:", sum(good_shape(a) for a in few_shot), "of 3")

print()
print("Tab 2: the accident")
print("words every example shares:", shared_accident(EXCHANGE_EXAMPLES))
print("varied examples share:", shared_accident(VARIED_EXAMPLES) or "nothing beyond the shape")
varied_answers = ["Verdict: low risk\n- funded by one wallet\n- 2 swaps\n- no exchange touched",
                  "Verdict: medium risk\n- funded through a bridge\n- 5 swaps\n- held for a week", "Not found"]
print("varied, shape ok:", sum(good_shape(a) for a in varied_answers), "of 3")
for n in (3, 20):
    cost = n * TOKENS_PER_EXAMPLE
    flag = "over budget" if cost > EXAMPLE_BUDGET else "ok"
    print(f"{n} examples: {cost} tokens on every call, {flag}")
