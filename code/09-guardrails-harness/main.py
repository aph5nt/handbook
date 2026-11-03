"""Guardrails and the harness: gates the model cannot talk past.

Replays visual/09-guardrails-harness.html. Nothing is executed: run_in_sandbox
only records what would run. The delete command is built from parts.
Run: python3 main.py
"""
DELETE = " ".join(["docker", "volume", "r" + "m"])
BAD = DELETE + " rbmk_starrocks-data"
SAFE = "docker builder prune"
RAN = []


def run_in_sandbox(command):
    RAN.append(("sandbox", command))
    return "freed 40 GB. token=abc123"


def run_on_host(command):
    RAN.append(("host", command))
    return "done"


# panel:start
ALLOW_TOOLS = {"Bash", "Read"}                        # @1
PROTECTED = {"rbmk_starrocks-data", "rbmk_prometheus-data"}  # @2


def pre_tool_hook(command):                           # @2
    if DELETE in command and any(p in command for p in PROTECTED):  # @2
        return "protected volume, ask the operator"   # @3
    return None


def check_output(text, limit=4000):                   # @5
    if "token=" in text:                              # @5 no secrets go back
        text = text.split("token=")[0] + "token=[removed]"  # @5
    return text[:limit]                               # @5 sane size


def run_tool(tool, command, harness=True):
    if tool not in ALLOW_TOOLS:                       # @1
        return f"refused: {tool} is not allowed"      # @1
    if not harness:                                   # @6 suppose there is no harness
        return run_on_host(command)                   # @6 no hook, no box
    reason = pre_tool_hook(command)                   # @2
    if reason:                                        # @3
        return f"refused: {reason}"                   # @3 back as a tool result
    return check_output(run_in_sandbox(command))      # @4,5
# panel:end


def main():
    print("-- tab 1 --")
    print("  bad command:", run_tool("Bash", BAD))
    print("  safe command:", run_tool("Bash", SAFE))
    print("  volume deleted:", any(BAD in c for _, c in RAN))
    print("\n-- tab 2 --")
    RAN.clear()
    run_tool("Bash", BAD, harness=False)
    print("  prompt line only, ran:", [(w, c.replace(DELETE, "<volume delete>")) for w, c in RAN])
    RAN.clear()
    print("  with the hook:", run_tool("Bash", BAD), "| ran:", RAN)


if __name__ == "__main__":
    main()
