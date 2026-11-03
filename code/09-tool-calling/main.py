"""Tool calling: the model writes a request, your code runs it.

Replays visual/09-tool-calling.html. fake_model picks calls by rule; a vague
description makes it guess. The schema check and the refusal are real code.
Run: python3 main.py
"""
ADDR = "0xabc0000000000000000000000000000000000abc"
BINANCE = "0xb1aa4b2c3d4e5f60718293a4b5c6d7e8f90a7e11"
SENDERS = ["0x18aa4b2c3d4e5f60718293a4b5c6d7e8f90acb24", "0x77aa4b2c3d4e5f60718293a4b5c6d7e8f90a30ff",
           "0xc2aa4b2c3d4e5f60718293a4b5c6d7e8f90a5518"]
GOOD_DESC = "Use when you know an address. For a name, call meta_dex_lookup first."
VAGUE_DESC = "Query the database."


def graph(statement, timeout_s):
    if "0x" not in statement:
        return {"error": "anchor_missing, give an address"}
    return {"rows": SENDERS}


def fake_model(question, description, known=None):
    if known:
        return ("graph_query", {"statement": f"MATCH senders of {known}", "network": "robinhood"})
    if ADDR in question:
        return ("graph_query", {"statement": f"MATCH senders of {ADDR}", "network": "robinhood"})
    if description == GOOD_DESC:
        return ("meta_dex_lookup", {"name": "Binance"})
    return ("graph_query", {"statement": "MATCH exchanges WHERE name = Binance", "network": "robinhood"})


# panel:start
SCHEMA = {"statement": str, "network": ("robinhood",)}   # @1
TIME_LIMIT_S = 10                                        # @4


def validate(args):                                      # @3
    for key, rule in SCHEMA.items():                     # @3
        if key not in args:                              # @3
            return f"missing {key}"                      # @3
        if isinstance(rule, tuple) and args[key] not in rule:  # @3
            return f"{key} must be one of {rule}"        # @3
    return None                                          # @3


def call_tool(name, args):
    if name == "meta_dex_lookup":                        # @7
        return {"address": BINANCE}                      # @7 the name, as an address
    problem = validate(args)                             # @3
    if problem:
        return {"error": problem}                        # @5
    return graph(args["statement"], timeout_s=TIME_LIMIT_S)  # @4 the host sets the limit


def answer(question, description):
    name, args = fake_model(question, description)       # @2 the model writes the call
    result = call_tool(name, args)                       # @3,4
    if "address" in result:                              # @7
        name, args = fake_model(question, description, known=result["address"])  # @7
        result = call_tool(name, args)                   # @7
    if "error" in result:                                # @5
        return f"tool_result: {result['error']}"         # @5 the model can recover
    return "senders: " + ", ".join(result["rows"])       # @6 tool_result, then words
# panel:end


def main():
    print("-- tab 1 --")
    print(answer(f"Who sent money to {ADDR}?", GOOD_DESC))
    print("\n-- tab 2 --")
    print("vague:", answer("Who sent money to Binance?", VAGUE_DESC))
    print("rule in the description:", answer("Who sent money to Binance?", GOOD_DESC))


if __name__ == "__main__":
    main()
