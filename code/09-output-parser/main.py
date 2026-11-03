"""Output parser: model text to a typed value, loudly.

Replays visual/09-output-parser.html. The model is a fake that answers in
prose on its first try for the second call, then in JSON once it sees the
error. Standard library only, no network. Run: python3 main.py
"""
import json
from dataclasses import dataclass

JSON_OK = '{"risk_kind": "mixer", "confidence": 0.8}'
PROSE = "Sure! It looks like a mixer."


def make_fake_model(answers):
    """Returns a model that gives the listed answers in order."""
    queue = list(answers)
    calls = []

    def model(prompt):
        calls.append(prompt)
        return queue.pop(0)
    model.calls = calls
    return model


# panel:start
KINDS = ("mixer", "exchange", "scam", "none")
FORMAT = 'Return only JSON: {"risk_kind": one of mixer|exchange|scam|none, "confidence": 0 to 1}'  # @1


@dataclass
class Verdict:
    risk_kind: str
    confidence: float


class ParseError(Exception):
    pass


def parse(text):
    try:
        data = json.loads(text)  # @3 syntax
    except json.JSONDecodeError:
        raise ParseError("expected JSON with risk_kind and confidence")  # @3
    if data.get("risk_kind") not in KINDS:  # @4 meaning, field by field
        raise ParseError(f"risk_kind must be one of {KINDS}")  # @4
    c = data.get("confidence")
    if not isinstance(c, (int, float)) or not 0 <= c <= 1:  # @4
        raise ParseError("confidence must be a number from 0 to 1")  # @4
    return Verdict(data["risk_kind"], float(c))  # @5 a typed value


def ask(model, prompt, retries=2):
    text = model(prompt + "\n" + FORMAT)  # @2
    for attempt in range(retries + 1):
        try:
            return parse(text)  # @3,4,5
        except ParseError as e:
            if attempt == retries:  # @7 retries spent
                raise ParseError(f"could not parse after {retries} retries: {e}")  # @7 never a default
            text = model(f"{prompt}\nYour answer: {text}\nError: {e}\n{FORMAT}")  # @6 retry with the reason


def ask_quietly(model, prompt):
    try:  # @8
        return parse(model(prompt + "\n" + FORMAT))  # @8 the model's text, parsed
    except ParseError:  # @8 any error at all
        return Verdict("none", 0.0)  # @8 WRONG: a default that looks like an answer

# better, where the provider has it: structured output, the schema  # @9
# constrains every token, so the bad text cannot be written          # @9
# panel:end


def main():
    print("-- tab 1: parse, or retry with the error --")
    m = make_fake_model([JSON_OK])
    print("call 1:", ask(m, "Screen wallet 0x9f2c4e7a1b3d5f6e8a0c2b4d6f8e0a1c3b5d0071"))
    m = make_fake_model([PROSE, JSON_OK])
    v = ask(m, "Screen wallet 0x9f2c4e7a1b3d5f6e8a0c2b4d6f8e0a1c3b5d0071")
    print(f"call 2: first answer {PROSE!r} -> retry {len(m.calls) - 1} of 2 with the error -> {v}")
    m = make_fake_model([PROSE, PROSE, PROSE])
    try:
        ask(m, "Screen it")
    except ParseError as e:
        print("call 3, prose three times:", e)

    print("-- tab 2: a parser that defaults quietly --")
    m = make_fake_model([PROSE])
    v = ask_quietly(m, "Screen it")
    print("quiet parser:", v, "-> labelled", "clean" if v.risk_kind == "none" else v.risk_kind)
    m = make_fake_model([PROSE, JSON_OK])
    print("loud parser:", ask(m, "Screen it"), "-> labelled mixer")


if __name__ == "__main__":
    main()
