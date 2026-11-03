"""Context engineering: choose what fills the window.

Replays visual/09-context-engineering.html. Token sizes are example numbers:
one tool description 400, one chunk 900. Window: 100,000 tokens.
Run: python3 main.py
"""
WINDOW = 100_000
INSTRUCTIONS = 10_000
TOOL = 400
CHUNK = 900
RECENT_AND_PINNED = 6_000
FULL_HISTORY = 19_000
QUESTION = 2_000


# panel:start
def fill(parts):
    used = 0
    for name, tokens in parts:
        used += tokens                               # @7
        print(f"  {name}: +{tokens:,} tokens, window {used:,} of {WINDOW:,}, {used * 100 // WINDOW} % full")  # @7
    return used


def window(on_purpose):  # on_purpose=False: lines 2, 3 and 4 are off
    return [("system instructions, cached", INSTRUCTIONS),                         # @1 stable parts first
            ("tools: 5 of 60", 5 * TOOL) if on_purpose else ("tools: all 60", 60 * TOOL),   # @2 only what this task needs
            ("facts: top 5 chunks, cited", 5 * CHUNK) if on_purpose  # @3
            else ("facts: top 50 chunks", 50 * CHUNK),                              # @3 a few, with sources
            ("recent turns + pinned facts", RECENT_AND_PINNED) if on_purpose  # @4
            else ("every past turn in full", FULL_HISTORY),                         # @4 older turns summarised
            ("the question, last", QUESTION)]                                      # @5


def fake_model(used):
    if used * 100 // WINDOW >= 90:                          # @6
        return "wrong: quotes a near-miss chunk"            # @6 noise pushed out the fact
    return "right: cites 2 of the 5 facts"                  # @6
# panel:end


def main():
    print(f"example sizes: one tool description {TOOL} tokens, one chunk {CHUNK} tokens")
    print("-- tab 1: filled on purpose --")
    a = fill(window(True))
    print("  answer:", fake_model(a))
    print("\n-- tab 2: filled by habit --")
    b = fill(window(False))
    print("  answer:", fake_model(b))
    print(f"\ntokens per call: {a:,} on purpose, {b:,} by habit, {b / a:.1f} times as many")


if __name__ == "__main__":
    main()
