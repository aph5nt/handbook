"""Token pricing: replay the lesson "Token pricing: input is cheap, output is not".

Prices are relative example units per 1,000 tokens, not a provider's list:
input 1, cached input 0.1, output 5. Reasoning tokens bill as output.
Standard library only, no network. Run: python3 main.py
"""

PRICE = {"input": 1.0, "cached": 0.1, "output": 5.0}   # per 1,000 tokens


def cost(usage):
    """One call's cost from its trace, split by token type."""
    return (usage["input"] / 1000 * PRICE["input"]
            + usage["cached"] / 1000 * PRICE["cached"]
            + (usage["reasoning"] + usage["output"]) / 1000 * PRICE["output"])


def show(label, usage):
    parts = {
        "input": usage["input"] / 1000 * PRICE["input"],
        "cached": usage["cached"] / 1000 * PRICE["cached"],
        "reasoning": usage["reasoning"] / 1000 * PRICE["output"],
        "output": usage["output"] / 1000 * PRICE["output"],
    }
    detail = ", ".join(f"{k} {v:.1f}" for k, v in parts.items())
    print(f"{label}: total {cost(usage):.1f}  ({detail})")
    return cost(usage)


# The traces of one call type, as in the lesson.
plan = {"input": 20_000, "cached": 0, "reasoning": 0, "output": 500}       # what the plan counted
before = {"input": 20_000, "cached": 0, "reasoning": 2_000, "output": 500}  # what the trace shows
cached = {"input": 5_000, "cached": 15_000, "reasoning": 2_000, "output": 500}
capped = {"input": 5_000, "cached": 15_000, "reasoning": 300, "output": 500}
after = {"input": 2_000, "cached": 15_000, "reasoning": 300, "output": 500}

print("Tab 1: the bill")
show("step 2, input only", {"input": 20_000, "cached": 0, "reasoning": 0, "output": 0})
show("step 3, plus 500 output", plan)
show("step 4, plus 2,000 reasoning", before)
print("   reasoning 10.0 is half of input 20.0, and it was not in the plan")

print()
print("Tab 2: cut it")
show("step 2, cache 15,000 of the input", cached)
show("step 3, thinking budget 2,000 -> 300", capped)
show("step 4, tool output trimmed to 2,000", after)
print(f"step 5: before {cost(before):.1f}, after {cost(after):.1f}")
