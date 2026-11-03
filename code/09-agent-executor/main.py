"""Agent executor: the loop code and its guards.

Replays visual/09-agent-executor.html. The model and the tools are fakes, so
the run is offline and prints the lesson's numbers. Sizes are in bytes of
text: 40 KB of rows, trimmed to 4 KB. Standard library only.
Run: python3 main.py
"""

QUESTION = "Where did wallet 0x9f2c4e7a1b3d5f6e8a0c2b4d6f8e0a1c3b5d0071 send its money?"


def fake_model_good(messages, tools):
    """Turn 1 asks for graph_query, turn 2 for lookup, turn 3 answers."""
    turn = sum(1 for m in messages if m["role"] == "tool") + 1
    if turn == 1:
        return {"tool": "graph_query", "args": {"address": QUESTION[18:60]}}
    if turn == 2:
        return {"tool": "lookup", "args": {"name": "Exchange Z"}}
    return {"text": "It reached Exchange A about 2 days later."}


def fake_model_stuck(messages, tools):
    return {"tool": "graph_query", "args": {"address": "?"}}  # never stops asking


def graph_query(address, size=40_000):
    return "row," * (size // 4)


def lookup(name):
    raise LookupError("name not found")


# panel:start
STEP_CAP = 8            # @1
RESULT_CAP = 4_000      # @7 4 KB
TOOLS = {"graph_query": graph_query, "lookup": lookup}


def run_tool(name, args, time_limit=10):
    if name not in TOOLS:  # @4 the name must be a known tool
        return f"unknown tool {name}"  # @4
    try:
        return TOOLS[name](**args)  # @5 a real executor runs it under time_limit seconds
    except Exception as e:  # @6
        return f"{name} failed: {e}"  # @6 an error the model can read


def execute(model, question, log):
    messages = [{"role": "user", "content": question}]
    for turn in range(1, STEP_CAP + 1):  # @1
        reply = model(messages, TOOLS)  # @2
        if "tool" not in reply:  # @3
            log.append(f"turn {turn}: answer returned")  # @3
            return reply["text"]  # @3
        result = run_tool(reply["tool"], reply["args"])  # @4,5
        if len(result) > RESULT_CAP:  # @7
            log.append(f"turn {turn}: result {len(result) // 1000} KB, trimmed to 4 KB")  # @7
            result = result[:RESULT_CAP] + " [trimmed, ask for a narrower query]"  # @7
        else:
            log.append(f"turn {turn}: {result[:40]}")
        messages.append({"role": "tool", "content": result})  # @8 the run's whole memory
    log.append(f"cap {STEP_CAP} reached: honest partial answer")  # @9
    return "Partial: the money left through graph rows I could not narrow down."  # @9
# panel:end


def execute_bare(model, question, window=200_000):
    """No guards: no cap (the bill stops it at 30), no trim, no error handling."""
    messages = [{"role": "user", "content": question}]
    for turn in range(1, 31):
        reply = model(messages, TOOLS)
        messages.append({"role": "tool", "content": graph_query("?", size=2_000_000)})
        while sum(len(m["content"]) for m in messages) > window and len(messages) > 1:
            messages.pop(0)  # the harness drops the oldest message to fit the window
        if turn == 1:
            print(f"window: {window // 1000} KB")
            print(f"turn 1: result {len(messages[-1]['content']) // 1_000_000} MB appended whole")
            still = any(m["role"] == "user" for m in messages)
            print("question still in the window:", still)
    print(f"turn {turn}: no cap, no answer")


def main():
    print("-- tab 1: loop with guards --")
    log = []
    print("answer:", execute(fake_model_good, QUESTION, log))
    print("\n".join(log))
    print("-- tab 2: loop with no guards --")
    execute_bare(fake_model_stuck, QUESTION)
    log = []
    print("fix, guards on:", execute(fake_model_stuck, QUESTION, log))
    print(log[-1])


if __name__ == "__main__":
    main()
