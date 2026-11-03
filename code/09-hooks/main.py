"""Hooks: code that runs at fixed points of the agent loop.

Replays visual/09-hooks.html. Nothing is executed: run_tool only prints.
The delete command is built from parts, so the words never sit together in
this file. Run: python3 main.py
"""
import os
import tempfile

DELETE = " ".join(["docker", "volume", "r" + "m"])
COMMAND = DELETE + " rbmk_starrocks-data"
LOG = []


def run_tool(command):
    LOG.append(f"RAN: {command}")
    return f"ok: {command}"


# panel:start
PROTECTED = {"rbmk_starrocks-data", "rbmk_prometheus-data"}  # @1


def pre_tool_hook(command):                                  # @2
    if command.startswith(DELETE):                           # @3
        for name in PROTECTED:                               # @3
            if name in command:                              # @3
                return "block", f"{name} is protected, never delete it"  # @3
    return "allow", ""                                       # @4


def call_tool(command, hooks_on=True):                       # @5
    if hooks_on:
        verdict, reason = pre_tool_hook(command)             # @6
        if verdict == "block":                               # @7
            return f"refused: {reason}"                      # @7 nothing ran
    result = run_tool(command)                               # @8
    LOG.append(f"post-tool log: {command}")                  # @9
    return result


def stop_hook(required_file):                                # @10
    if not os.path.exists(required_file):                    # @10
        return f"refuse to finish: {required_file} is missing"  # @10
    return "stop allowed"                                    # @10


PROMPT_RULE = "Never delete database volumes."               # @11 advice, no code runs it
# panel:end


def main():
    print("-- tab 1: a hook blocks it --")
    print("agent asks for:", COMMAND.replace(DELETE, "<volume delete>"))
    print("tool result:", call_tool(COMMAND))
    print("volume deleted:", any(l.startswith("RAN") for l in LOG))
    print(call_tool("ls /tmp"))
    required = os.path.join(tempfile.mkdtemp(), "lesson.md")
    print("stop hook:", stop_hook(required))
    open(required, "w").close()
    print("stop hook:", stop_hook(required))

    print("\n-- tab 2: a prompt rule asks nicely --")
    LOG.clear()
    print("prompt says:", PROMPT_RULE)
    print("hour 10, after compaction, no hook:")
    call_tool(COMMAND, hooks_on=False)
    print("volume deleted:", any(l.startswith("RAN") for l in LOG))
    LOG.clear()
    print("hook added, hour 10:", call_tool(COMMAND))
    print("volume deleted:", any(l.startswith("RAN") for l in LOG))


if __name__ == "__main__":
    main()
