"""Chains, runnables and LCEL, in plain Python.

Replays visual/09-lcel-chain.html. A Runnable has invoke(); `a | b` makes a
new Runnable. The model is a fake: it answers in JSON, except for one log
that makes it answer in prose. Standard library only, no network.
Run: python3 main.py
"""
import json
import traceback


def fake_model(prompt):
    if "disk full" in prompt:  # this log makes the model chat instead of answering in JSON
        return "Sure! This looks like a disk problem."
    return json.dumps({"summary": "alerts from one indexer", "risk": "high"})


# panel:start
TRACE = []          # the trace: each runnable's input and output
TRACING = True      # @9 off by default in many setups


class Runnable:
    def __init__(self, name, fn):
        self.name, self.fn = name, fn  # @1 invoke(input) -> output

    def invoke(self, x):
        try:
            out = self.fn(x)  # @1
        except Exception as e:
            if TRACING:
                TRACE.append((self.name, x, f"ERROR {e}"))  # @10 the trace names the step
            raise
        if TRACING:
            TRACE.append((self.name, x, out))  # @9
        return out

    def __or__(self, other):  # @2 a | b runs a, then b on a's output
        return Runnable(f"{self.name} | {other.name}", lambda x: other.invoke(self.invoke(x)))  # @2

    def batch(self, xs):
        return [self.invoke(x) for x in xs]  # @3

    def with_retry(self, attempts=2):
        def run(x):
            for k in range(attempts):  # @3
                try:
                    return self.invoke(x)
                except ValueError:
                    if k == attempts - 1:
                        raise
        return Runnable(self.name + " (retry)", run)


def parse_json(text):
    try:
        return json.loads(text)  # @6
    except json.JSONDecodeError:
        raise ValueError(f"not JSON: {text!r}")  # @6 raise on bad text


prompt = Runnable("prompt", lambda d: f"Summarise this incident log: {d['log']}")  # @4
model = Runnable("model", fake_model)  # @5
parser = Runnable("parser", parse_json)  # @6
chain = prompt | model | parser  # @7 the chain is a Runnable too
# panel:end


def main():
    global TRACING
    print("-- tab 1: prompt, model, parser --")
    out = chain.invoke({"log": "2,000 words of alerts"})  # pseudo line 8
    for name, x, y in TRACE:
        if "|" not in name:  # skip the composed chains, show the three steps
            print(f"  {name}: {str(x)[:45]!r} -> {str(y)[:45]!r}")
    print("typed result:", out)
    TRACE.clear()
    TRACING = False
    logs = [{"log": f"incident {k}"} for k in range(30)]
    print("batch of 30:", len(chain.batch(logs)), "results")
    print("with_retry:", chain.with_retry().invoke({"log": "incident 31"})["risk"])

    print("-- tab 2: the error hides inside the chain --")
    deep = (Runnable("clean", lambda d: d) | Runnable("redact", lambda d: d)) | (prompt | (model | parser)) | Runnable("label", lambda d: d)  # six runnables, nested
    try:
        deep.invoke({"log": "disk full on the indexer"})
    except ValueError as e:
        frames = traceback.extract_tb(e.__traceback__)[1:]  # skip main() itself
        lambdas = sum(1 for f in frames if f.name == "<lambda>")
        print(f"tracing off, the error alone: {e.__class__.__name__} through {lambdas} nested lambdas, "
              f"{len(frames)} frames, no prompt shown")
    TRACING = True
    try:
        deep.invoke({"log": "disk full on the indexer"})
    except ValueError:
        for name, x, y in TRACE:
            if name in ("prompt", "model", "parser"):
                print(f"  trace {name}: in={str(x)[:40]!r} out={str(y)[:40]!r}")


if __name__ == "__main__":
    main()
