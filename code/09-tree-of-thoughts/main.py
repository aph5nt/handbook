"""Reflexion and tree of thoughts.

Replays visual/09-tree-of-thoughts.html. The real check is a table of how much
money took each path (a stand-in for a graph query). The fake model proposes
ideas and, when asked its opinion, finds every idea promising.
Run: python3 main.py
"""
IDEAS = {
    "start": ["A: via the bridge", "B: via the swap pool", "C: via the mixer"],
    "B: via the swap pool": ["B1: pool to exchange", "B2: pool to new wallet"],
}
MONEY_SHARE = {"A: via the bridge": 0.1, "B: via the swap pool": 0.8, "C: via the mixer": 0.1,
               "B1: pool to exchange": 0.7, "B2: pool to new wallet": 0.1}


def fake_propose(node):
    return IDEAS.get(node, [])


def real_check(idea):
    return MONEY_SHARE[idea]


def model_opinion(idea):
    return 0.9  # "promising": the model likes what it wrote


# panel:start
WIDTH, MAX_DEPTH, PRUNE = 3, 2, 0.5                  # @2,5


def tree_search(score):
    frontier, best, calls = ["start"], None, 0       # @1
    for depth in range(1, MAX_DEPTH + 1):            # @2 depth limit
        children = []
        for node in frontier:
            calls += 1
            children += fake_propose(node)[:WIDTH]   # @3 width limit
        scored = []
        for child in children:
            calls += 1
            scored.append((score(child), child))     # @4 a real check, not an opinion
            print(f"  depth {depth}: {child} scores {scored[-1][0]}")  # @4
        kept = [s for s in scored if s[0] >= PRUNE]  # @5 prune
        if not kept:                                 # @6
            break                                    # @6
        frontier = [c for _, c in kept]              # @5
        best = max(kept)[1]                          # @7
    return best, calls                               # @7


def reflexion(attempts, check):                      # @8
    notes = []                                       # @8
    for attempt in attempts:                         # @8
        if check(attempt, notes):                    # @8
            return attempt, notes                    # @8
        notes.append(f"{attempt} was a dead end")    # @8 the lesson
    return None, notes
# panel:end


def calls_with_no_limit(width, depth):
    return sum(width ** d for d in range(1, depth + 1))


def main():
    print("-- tab 1: branch, score, prune --")
    best, calls = tree_search(real_check)
    print(f"  answer: {best}, {calls} calls")
    found, notes = reflexion(["the bridge", "the swap pool"], lambda a, n: a == "the swap pool")
    print(f"  reflexion: found '{found}', notes {notes}")

    print("\n-- tab 2: a tree with no limit --")
    print("  opinions:", {i: model_opinion(i) for i in IDEAS["start"]}, "nothing is pruned")
    for d in range(1, 5):
        print(f"  depth {d}: {3 ** d} branches, {calls_with_no_limit(3, d)} scoring calls so far")
    print("  the fix, real check with width 3 and depth 2:", tree_search(real_check)[1], "calls")


if __name__ == "__main__":
    main()
