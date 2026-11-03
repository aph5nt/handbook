"""Conditional edges: code picks the next node.

Replays visual/09-conditional-edges.html. Standard library only.
Run: python3 main.py
"""


def approve(state):
    return "approved"


def review(state):
    return "sent to a person"


def block(state):
    return "blocked"


# panel:start
def score(state):                                  # @1
    state["risk_score"] = state["input_score"]     # @1 a model or a rule writes this


def route(state):                                  # @2
    if state.get("sanctioned"):                    # @3 the new result
        return "sanctioned"                        # @3
    if state["risk_score"] < 0.3:                  # @4
        return "approve"                           # @4
    if state["risk_score"] <= 0.7:                 # @5
        return "human review"                      # @5
    return "block"                                 # @6


PATH_MAP = {"approve": approve, "human review": review, "block": block}  # @7


def run(state, safe_default=True):
    score(state)
    name = route(state)                            # @8
    if safe_default:
        node = PATH_MAP.get(name, block)           # @9 unknown name: fail safe
    else:
        node = PATH_MAP[name]                      # @12 suppose the default is off
    return name, node(state)                       # @10 run the node, then END


def test_routes():                                 # @11
    states = [{"risk_score": 0.1}, {"risk_score": 0.55}, {"risk_score": 0.9},
              {"risk_score": 0.5, "sanctioned": True}]
    return [route(s) for s in states if route(s) not in PATH_MAP]  # @11
# panel:end


def main():
    print("-- tab 1: route by the state --")
    name, result = run({"input_score": 0.55})
    print(f"risk_score 0.55 -> route returns '{name}' -> {result}, then END")
    print("path map names:", sorted(PATH_MAP))

    print("\n-- tab 2: a route with no node --")
    wallet = {"input_score": 0.5, "sanctioned": True}
    try:
        run(dict(wallet), safe_default=False)
    except KeyError as e:
        print(f"default off: route returned {e}, no such node, the run stops half done")
    name, result = run(dict(wallet))
    print(f"default on: route returned '{name}' -> {result}")
    print("test: names with no node:", test_routes())


if __name__ == "__main__":
    main()
