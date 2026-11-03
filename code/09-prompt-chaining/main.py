"""Prompt chaining: small calls in a fixed order, with a gate in code.

Replays visual/09-prompt-chaining.html. fake_model returns canned text, and
drops the risk level on the first try of the "forgetful" day.
Run: python3 main.py
"""
LOG = "about 2,000 words of alerts and notes"
calls = 0


def fake_model(prompt, forgetful=False):
    global calls
    calls += 1
    if prompt.startswith("Summarise"):
        if forgetful and "name a risk level" not in prompt:
            return "Disk filled on lane B and the sync stopped."
        return "Disk filled on lane B and the sync stopped. risk: high"
    if prompt.startswith("Translate"):
        return "[translated] " + prompt.split(": ", 1)[1]
    text = prompt.split(": ", 1)[1]
    risk = "high" if "risk: high" in text else ""
    return f"| time | service | impact | risk |\n| 09:00 | lane B | sync stopped | {risk} |"


# panel:start
RETRIES = 2                                            # @5


def gate(summary):                                     # @3
    return "risk:" in summary                          # @3 plain code, no model


def run_chain(log, forgetful=False, use_gate=True):  # @1
    reason = ""
    for attempt in range(RETRIES + 1):                 # @5
        summary = fake_model(f"Summarise {log}{reason}", forgetful)  # @2
        if not use_gate or gate(summary):              # @3
            break                                      # @3
        print(f"  gate: no risk level, retry {attempt + 1} of {RETRIES}")  # @4
        reason = ". You must name a risk level"        # @4 back with the reason
    else:
        return "stopped: no risk level after retries"  # @5
    translated = fake_model(f"Translate: {summary}")   # @6
    return fake_model(f"Format as a table: {translated}")  # @7,8
# panel:end


def main():
    global calls
    print("-- tab 1: a chain with a gate --")
    print("input:", LOG)
    print(run_chain(LOG))
    print(f"model calls: {calls}")

    print("\n-- tab 2: no gate, then the fix --")
    calls = 0
    print(run_chain(LOG, forgetful=True, use_gate=False))
    print(f"no gate: the risk column is empty, {calls} model calls")
    calls = 0
    print(run_chain(LOG, forgetful=True))
    print(f"gate on: risk column filled, {calls} model calls (one extra)")


if __name__ == "__main__":
    main()
