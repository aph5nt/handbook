"""Tracing a slow answer: a tiny span tree and a redact-before-export step.

Tab 2 of visual/09-evals-tracing.html. The clock is simulated, so the run is
instant and prints the lesson's numbers (example numbers: 14 s in all).
Run it through main.py, or alone: python3 tracing.py
"""
import contextlib
import re

NOW = [0.0]  # simulated clock, seconds
SPANS = []
STACK = []


def work(seconds):
    NOW[0] += seconds


# panel:start
@contextlib.contextmanager
def span(name, **attrs):
    s = {"name": name, "parent": STACK[-1]["name"] if STACK else None,  # @5 parent = current span
         "start": NOW[0], "attrs": attrs}  # @5 record the start
    STACK.append(s)
    try:
        yield s
    finally:
        STACK.pop()
        s["end"] = NOW[0]  # @5 end on exit
        SPANS.append(s)


def handle_request(prompt):
    with span("request", prompt=prompt):  # @1 one tree per request
        with span("model call 1", tokens=900):  # @2
            work(2)
        with span("graph_query", note="timed out, retried"):  # @3
            work(9)
        with span("model call 2", tokens=1200):  # @4
            work(3)


def slowest():
    kids = [s for s in SPANS if s["parent"]]
    return max(kids, key=lambda s: s["end"] - s["start"])  # @6 the span to fix


ADDRESS = re.compile(r"0x[0-9a-fA-F]{40}")


def export(spans, exporter, redact):
    if exporter == "third party" and not redact:  # @7
        return "prompts and addresses sent to a third party"  # @8
    for s in spans:  # @9
        if "prompt" in s["attrs"]:  # @9
            s["attrs"]["prompt"] = ADDRESS.sub("<address>", s["attrs"]["prompt"])  # @9 redact addresses
    return f"exported {len(spans)} spans to {exporter}, prompts redacted"  # @9
# panel:end


def main():
    handle_request("Where did 0x9f2c4e7a1b3d5f6e8a0c2b4d6f8e0a1c3b5d0071 send its money?")
    for s in sorted(SPANS, key=lambda s: (s["parent"] is not None, s["start"])):
        pad = "  " if s["parent"] else ""
        print(f"{pad}{s['name']}: {s['end'] - s['start']:.0f} s")
    sl = slowest()
    print(f"slowest span: {sl['name']} ({sl['end'] - sl['start']:.0f} s, {sl['attrs']['note']})")
    print("export, no decision:", export([dict(s, attrs=dict(s["attrs"])) for s in SPANS], "third party", False))
    print("export, decided first:", export(SPANS, "self-hosted collector", True))


if __name__ == "__main__":
    main()
