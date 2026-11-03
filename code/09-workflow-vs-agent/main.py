"""Workflow versus agent: who picks the next step.

Replays visual/09-workflow-vs-agent.html. fake_model is a script: the workflow
gets canned text, the agent gets a canned list of tool choices per morning. The
agent's call counts are example numbers. Run: python3 main.py
"""
ALERTS = [f"alert {i}" for i in range(1, 13)]
calls = 0
gaps = [True, False, False, True, False]  # which of the 5 mornings the gate fires

# One list per morning: which tools the fake agent picks before it answers.
MORNINGS = [["read"] * 6 + ["runbook", "runbook", "logs"], ["read"] * 6, ["read"] * 6 + ["logs"] * 5,
            ["read"] * 8 + ["runbook"] * 3 + ["logs"] * 3, ["read"] * 6 + ["runbook"] * 2]
NEW_QUESTION = ["logs", "logs", "runbook", "logs", "runbook"]


def fake_model(prompt, *_):
    global calls
    calls += 1
    if prompt.startswith("fix"):
        return "12 lines, all with a risk level"
    if prompt.startswith("summarise"):
        # On some mornings one line comes back with no risk level.
        return "12 lines, one with no risk level" if gaps.pop(0) else "12 lines, all with a risk level"
    return "a table for the stand-up page"


# panel:start
def workflow(alerts):                                    # @1
    summary = fake_model("summarise, one line and a risk level each", alerts)  # @2
    if "no risk level" in summary:                       # @3 the gate, plain code
        summary = fake_model("fix: a line has no risk level", summary)  # @3
    return fake_model("format as a table", summary)      # @4


def agent(goal, script, cap=20):                         # @5
    global calls
    messages = [goal]                                    # @6
    for tool in script[:cap]:                            # @7
        calls += 1                                       # @8 the model picks the next step
        messages.append(f"result of {tool}")             # @10
    calls += 1                                           # @8
    return f"answer after reading {len(messages) - 1} results"  # @9
# panel:end


def main():
    global calls
    print("-- tab 1: workflow --")
    for morning in range(1, 6):
        calls = 0
        print(f"morning {morning}: {workflow(ALERTS)}, {calls} calls")
    print("new question: the workflow has no step for it, it summarises again")

    print("\n-- tab 2: agent --")
    for morning, script in enumerate(MORNINGS, 1):
        calls = 0
        agent("one page on 12 alerts", script)
        print(f"morning {morning}: {calls} calls")
    calls = 0
    print("new question:", agent("why did alert 7 fire twice?", NEW_QUESTION), f"({calls} calls)")


if __name__ == "__main__":
    main()
