"""Hallucination and grounding: replay the lesson "grounding beats scolding".

Tab 1 answers from model memory: a fluent address that does not exist.
Tab 2 answers from a lookup tool, and a checker in code matches every address
in the draft against the store before it goes out. "Not found" is allowed.
The model is a stand-in: fixed text, so the run is the same every time.
Standard library only. Run: python3 main.py
"""
import re

WALLET = "0x9f3c4e8a1b2d7f6c5e0a9b8d7c6f5e4d3a2b0071"
WALLET2 = "0x18ab52e7c9d04f1a3b6e8d2c5f7a9b0e1d3ccb24"
POOL = "0x5c1d7e2a9f4b6c8d0e3a5f7b9c1d2e4f6a8ba830"
DAY = "2026-11-01"

# The graph store: which contracts exist, and the swaps of each wallet per day.
CONTRACTS = {POOL, "0x4a2b9c8d7e6f5a4b3c2d1e0f9a8b7c6d5e4fee02"}
SWAPS = {(WALLET, DAY): [{"pool": POOL, "block_time": "2026-11-01 14:07"}]}

# What the model "remembers": pool-shaped strings it saw in training.
MEMORY_GUESS = {"plain": "0x7a21b3c4d5e6f708192a3b4c5d6e7f8091a2c4e9",
                "shout": "0x3be0f1e2d3c4b5a69788796a5b4c3d2e1f0091d2"}

ADDRESS = re.compile(r"0x[0-9a-f]{40}")


def answer_from_memory(prompt):
    """No tool: the most likely text. Right shape, never looked up."""
    return f"It sold into pool {MEMORY_GUESS[prompt]}."


def lookup(wallet, day):
    """The tool: rows from the store, the pool in a typed field."""
    return SWAPS.get((wallet, day), [])


def check(draft, read_from_tools):
    """Every address in the draft must exist and must come from a tool result."""
    bad = []
    for addr in ADDRESS.findall(draft):
        if addr == WALLET or addr == WALLET2:
            continue                                  # the question's own address
        if addr not in CONTRACTS or addr not in read_from_tools:
            bad.append(addr)
    return bad


def grounded_answer(wallet, day):
    rows = lookup(wallet, day)
    if not rows:
        return "No swap found for this wallet that day. Give another address or day."
    seen = {r["pool"] for r in rows}
    draft = f"Sold into pool {rows[0]['pool']} (source: swap row, {rows[0]['block_time']})."
    bad = check(draft, seen)
    return f"REFUSED: {bad}" if bad else draft


print("Tab 1: no grounding")
for prompt in ("plain", "shout"):
    reply = answer_from_memory(prompt)
    guess = ADDRESS.findall(reply)[0]
    print(f"  {prompt:5}: {reply}")
    print(f"         exists in the store? {guess in CONTRACTS}   checker says: refuse {check(reply, set())}")

print()
print("Tab 2: grounded")
print(f"  lookup({WALLET[:6]}..., {DAY}) -> {len(lookup(WALLET, DAY))} row")
print(f"  {grounded_answer(WALLET, DAY)}")
print(f"  lookup(wallet 2) -> {len(lookup(WALLET2, DAY))} rows")
print(f"  {grounded_answer(WALLET2, DAY)}")
