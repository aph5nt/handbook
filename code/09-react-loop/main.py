"""The ReAct loop: think, act, observe, repeat.

Replays visual/09-react-loop.html. fake_model is a script, one reply per round.
The guards are real code. Run: python3 main.py
"""
SELLER = "0x9f2c4e7a1b3d5f6e8a0c2b4d6f8e0a1c3b5d0071"
EMPTY = "0x77aa4b2c3d4e5f60718293a4b5c6d7e8f90a30ff"
POOL = "0x4a7d2e9c0b1f3a5d7c9e1b3f5a7c9e1b3f5aee02"


def run_tool(name, arg):
    if name == "graph_query":
        return f"14 swaps into pool {POOL}, 2026-10-30" if arg == SELLER else "0 rows"
    return "Aerodrome, USDC/WETH pool"


def good_script(messages):
    round_no = sum(1 for m in messages if m.startswith("observe"))
    if round_no == 0:
        return "I need the swaps of this address.", ("graph_query", SELLER)
    if round_no == 1:
        return "A pool address is not an exchange name.", ("meta_dex_lookup", POOL)
    return "I am done.", ("answer", f"The address sold into Aerodrome, through its USDC/WETH pool {POOL}, in 14 swaps on 30 October.")


def stubborn_script(messages):
    if any("repeated call blocked" in m for m in messages):
        return "I must answer.", ("answer", f"The graph holds no swaps for {EMPTY}. Either the address never sold on a DEX, or the data is behind. Give me a transaction hash to check.")
    return "Maybe it failed. Try again.", ("graph_query", EMPTY)


# panel:start
MAX_ROUNDS = 8                                          # @2


def react(question, model, detect_repeats=True):
    messages = ["system prompt", "tool list", question]  # @1
    seen = set()
    for round_no in range(1, MAX_ROUNDS + 1):           # @2
        thought, action = model(messages)               # @3
        if action[0] == "answer":                       # @4
            return round_no, action[1]                  # @4
        if detect_repeats and action in seen:           # @5 same tool, same arguments
            observation = "repeated call blocked, change the plan or answer"  # @5
        else:
            observation = run_tool(*action)             # @6
        seen.add(action)
        messages += [f"think: {thought}", f"act: {action[0]}", f"observe: {observation}"]  # @7
        print(f"  round {round_no}: {action[0]} -> {observation[:90]}")
    return MAX_ROUNDS, "stopped at the cap: best answer so far"  # @8
# panel:end


def main():
    print("-- tab 1 --")
    rounds, answer = react(f"Which exchange did {SELLER} sell into?", good_script)
    print(f"  answer in round {rounds}: {answer}")
    print("\n-- tab 2 --")
    print("no repeat detector:")
    rounds, answer = react(f"Which exchange did {EMPTY} sell into?", stubborn_script, detect_repeats=False)
    print(f"  {answer}")
    print("with the repeat detector:")
    rounds, answer = react(f"Which exchange did {EMPTY} sell into?", stubborn_script)
    print(f"  answer in round {rounds}: {answer}")


if __name__ == "__main__":
    main()
