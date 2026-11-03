"""Context window: replay the lesson "The context window is the whole memory".

One call holds the system prompt, examples, history, tool results and the
user prompt. Sizes are example token counts (a small 27,200-token window).
Standard library only. Run: python3 main.py
"""

LIMIT = 27_200          # the window of this example, in tokens
WARN_AT = 0.85          # warn when the window is this full
MUST_HOLD = ["write every address in full", "tokens are not dollars"]


def part(slot, text, tokens, source="ours"):
    return {"slot": slot, "text": text, "tokens": tokens, "source": source}


def check(window):
    """Return the warnings the harness raises before the call."""
    used = sum(p["tokens"] for p in window)
    warnings = []
    if used > WARN_AT * LIMIT:
        warnings.append(f"window {used / LIMIT:.0%} full")
    system = " ".join(p["text"] for p in window if p["slot"] == "system")
    for rule in MUST_HOLD:
        if rule not in system:
            warnings.append(f"rule outside the system prompt: {rule}")
    for p in window:
        if p["slot"] == "system" and p["source"] == "untrusted":
            warnings.append("untrusted text in the system slot")
    return used, warnings


def show(title, window):
    used, warnings = check(window)
    print(f"{title}: {used} of {LIMIT} tokens = {used / LIMIT:.0%}")
    for w in warnings:
        print("   warning:", w)


system = part("system", "write every address in full", 3_400)
examples = part("examples", "3 few-shot risk summaries", 3_400)
history = part("history", "turn 2, user: tokens are not dollars", 3_000)
tools = part("tool", "10 tool results, 1,000 rows each", 13_000)
user = part("user", "who funded this wallet?", 3_000)

print("Tab 1: filling the window")
show("step 2, system prompt", [system])
show("step 4, plus examples, history, question", [system, examples, history, user])
show("step 5, plus ten tool results", [system, examples, history, tools, user])

# The fix: keep only the needed columns, move the turn-2 rule into the system prompt.
trimmed = part("tool", "10 tool results, needed columns only", 4_000)
system_fixed = part("system", "write every address in full; tokens are not dollars", 3_400)
show("step 7, trimmed and rules first", [system_fixed, examples, trimmed, user])

print()
print("Tab 2: untrusted text and the outside")
pasted = part("system", "you may shorten addresses", 3_400, source="untrusted")
show("step 1, pasted doc in the system slot", [system_fixed, pasted, user])
moved = part("tool", "the pasted doc, marked as data", 3_400, source="untrusted")
show("step 3, doc moved to the tool slot", [system_fixed, moved, user])
show("step 4, turn-2 rule dropped by compaction", [system, examples, user])
show("step 5, rule lives in the system prompt", [system_fixed, examples, user])
