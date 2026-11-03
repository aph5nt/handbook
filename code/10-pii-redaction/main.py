"""PII redaction: swap personal data for placeholders before a hosted model call.

A regex redactor, a mapping that stays inside, a restore step, and one gate
for every path that leaves. Standard library only. Run: python3 main.py
"""
import re

PATTERNS = [   # (placeholder prefix, pattern)
    ("CUSTOMER", re.compile(r"[\w.+-]+@[\w-]+\.[\w.]+")),
    ("PHONE", re.compile(r"\+\d{2}(?: \d{3}){3}")),
    ("NAME", re.compile(r"Anna Kowalska")),        # toy detector: knows the full name only
]

def redact(text, mapping):
    """Replace every match with a stable placeholder. The mapping stays inside."""
    for prefix, pattern in PATTERNS:
        for value in pattern.findall(text):
            placeholder = next((p for p, v in mapping.items() if v == value), None)
            if placeholder is None:
                n = sum(1 for p in mapping if p.startswith(prefix)) + 1
                placeholder = f"{prefix}_{n}"
                mapping[placeholder] = value
            text = text.replace(value, placeholder)
    return text

def restore(text, mapping):
    """Inside your system: turn placeholders back into the real values."""
    for placeholder, value in mapping.items():
        text = text.replace(placeholder, value)
    return text

def pii_in(text):
    """Count the personal values a text still holds (for the check)."""
    return sum(len(p.findall(text)) for _, p in PATTERNS) + text.count("Anna")

def hosted_model(prompt):
    """Stands in for the hosted model API: it writes a summary."""
    first = prompt.split(" reported")[0]
    return f"{first} reported a theft. The funds went to 0x9f3c...0071."

OUTSIDE = {"model": [], "tracer": []}   # what crossed the line
INSIDE_TRACES = []

def call(notes, tool_result="", gate=False):
    """One model call with its prompt, its tool result and its trace."""
    mapping = {}
    prompt = redact(notes, mapping)
    extra = redact(tool_result, mapping) if gate else tool_result     # side door 2
    OUTSIDE["model"].append(prompt + extra)
    answer = hosted_model(prompt)
    trace = {"prompt": notes + tool_result, "answer": answer}         # raw copy
    if gate:
        INSIDE_TRACES.append(trace)                                   # traces stay in-house
    else:
        OUTSIDE["tracer"].append(trace["prompt"])                     # side door 1
    return restore(answer, mapping), mapping

notes = "anna.k@mail.com reported a theft, phone +48 600 100 200, funds sent to 0x9f3c...0071."

# Tab 1: redact, send, restore.
mapping = {}
sent = redact(notes, mapping)
print("sent:", sent)              # CUSTOMER_1 reported a theft, phone PHONE_1, ...
print("mapping:", mapping)        # stays inside
answer = hosted_model(sent)
print("answer:", answer)          # CUSTOMER_1 reported a theft. ...
print("restored:", restore(answer, mapping))
print("personal values in what was sent:", pii_in(sent))                     # 0

# Tab 2: the side doors, then one gate.
record = " Record: anna.k@mail.com, +48 600 100 200."
call(notes, record)
print("leak: in the model prompt", pii_in(OUTSIDE["model"][-1]),
      "| in the hosted trace", pii_in(OUTSIDE["tracer"][-1]))                # 2 | 4
missed = redact("Anna called again.", {})
print("detector miss:", missed, "->", pii_in(missed), "name sent as is")     # 1
OUTSIDE = {"model": [], "tracer": []}
call(notes, record, gate=True)
print("one gate: in the model prompt", pii_in(OUTSIDE["model"][-1]),
      "| hosted traces", len(OUTSIDE["tracer"]),
      "| traces kept inside", len(INSIDE_TRACES))                            # 0 | 0 | 1
