"""Structured output: mask the tokens that would break the schema.

Replays visual/09-structured-output.html. A tiny fake model scores candidate
tokens. The mask is real: it drops every token the schema does not allow, and
the sampler takes the best one left. Run: python3 main.py
"""
import json

SCHEMA = {"risk_kind": ["mixer", "exchange", "scam", "none"], "confidence": "number"}

# The fake model's scores for the next token, per field. The chatty words score highest.
SCORES = {
    "risk_kind": {"probably": 0.5, "Sure": 0.3, "mixer": 0.15, "exchange": 0.05},
    "confidence": {"high": 0.6, '"very"': 0.2, "0.8": 0.2},
}


def is_number(tok):
    try:
        float(tok)
        return True
    except ValueError:
        return False


# panel:start
def allowed(field, token):                             # @2
    rule = SCHEMA[field]                               # @1 the schema travels with the request
    return is_number(token) if rule == "number" else token in rule  # @2


def constrained_decode():
    out = {}
    for field in SCHEMA:                               # @3 field by field
        scores = SCORES[field]                         # @4 the model scores candidates
        valid = {t: s for t, s in scores.items() if allowed(field, t)}  # @5 the mask
        masked = [t for t in scores if t not in valid]
        token = max(valid, key=valid.get)              # @6 best token left
        print(f"  {field}: masked {masked}, picked {token}")
        out[field] = float(token) if SCHEMA[field] == "number" else token  # @6
    return json.dumps(out)                             # @7 always a complete object


def act(text):                                         # @8
    verdict = json.loads(text)                         # @8 safe after the mask, fails on chat text
    branch = {"mixer": "flag it", "exchange": "name it",
              "scam": "warn", "none": "pass"}          # @8 one branch per value
    return branch[verdict["risk_kind"]], verdict       # @8


def eval_case(verdict, truth):                         # @9
    return verdict["risk_kind"] == truth               # @9 shape valid is not truth
# panel:end


def main():
    print("-- tab 1 --")
    for field, scores in SCORES.items():
        print(f"  {field} scores: {scores}")
    text = constrained_decode()
    print("  output:", text)
    print("  code:", act(text)[0])

    print("\n-- tab 2 --")
    chatty = "Sure! Here is the JSON: {risk: probably a mixer}"
    try:
        json.loads(chatty)
    except json.JSONDecodeError as e:
        print("  prompt line only:", chatty, "->", "parse error:", e.msg)
    text2 = '{"risk_kind": "likely_mixer"}'
    try:
        act(text2)
    except KeyError as e:
        print("  retry parses, no branch for", e)
    print("  with the mask, likely_mixer allowed:", allowed("risk_kind", "likely_mixer"))
    wrong = {"risk_kind": "none", "confidence": 0.9}
    print("  model returns:", json.dumps(wrong))
    print("  shape valid:", allowed("risk_kind", wrong["risk_kind"]), "| eval on a known mixer:", eval_case(wrong, "mixer"))


if __name__ == "__main__":
    main()
