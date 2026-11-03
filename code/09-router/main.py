"""Router: one cheap classification picks one fixed path.

Replays visual/09-router.html. fake_small_model is a word rule, and it guesses
how_to on a question it cannot place, like the real failure.
Run: python3 main.py
"""
import re

LABELS = ["address", "exchange_name", "how_to", "unsure"]


def fake_small_model(question):
    q = question.lower()
    if "aerodrome" in q:
        return "exchange_name", 0.95
    if q.startswith("how do i"):
        return "how_to", 0.9
    if "exchange" in q:
        return "how_to", 0.4        # sounds general, and it is not sure
    return "unsure", 0.2


def path_a(q):
    return "graph: the role of the address"


def path_b(q):
    return "lookup pools, graph_query anchored on them: the sellers, named in full"


def path_c(q):
    return "docs search: a cited answer"


# panel:start
PATHS = {"address": path_a, "exchange_name": path_b, "how_to": path_c}  # @5
ADDRESS = re.compile(r"\b0x[0-9a-fA-F]{40}\b")          # @1


def route(question, use_rule=True, min_confidence=0.7):
    if use_rule and ADDRESS.search(question):           # @1 a rule before the model
        return "address"                                # @1
    label, confidence = fake_small_model(question)      # @2 one label, fixed list
    if use_rule and confidence < min_confidence:        # @3
        return "unsure"                                 # @3
    return label                                        # @4


def answer(question, **kw):
    label = route(question, **kw)
    if label == "unsure":                               # @6
        return label, "ask the user one short question"  # @6
    return label, PATHS[label](question)                # @5 fixed code per label


def accuracy(labelled):                                 # @7
    hits = sum(route(q) == want for q, want in labelled)  # @7
    return f"{hits} of {len(labelled)}"                 # @7
# panel:end


def main():
    addr = "0x9f2c4e7a1b3d5f6e8a0c2b4d6f8e0a1c3b5d0071"
    print("-- tab 1 --")
    for q in ["Who sold into Aerodrome?", "How do I top up my wallet?"]:
        print(q, "-> model says", fake_small_model(q), "->", answer(q))
    print("\n-- tab 2 --")
    q = f"Is {addr} an exchange?"
    print("no rule, forced label:", fake_small_model(q), answer(q, use_rule=False, min_confidence=0))
    print("rule first:", answer(q))
    print("no address, not sure:", fake_small_model("Is this one an exchange?"), "0.4 < 0.7 ->", answer("Is this one an exchange?"))
    print("router eval:", accuracy([(q, "address"), ("Who sold into Aerodrome?", "exchange_name"),
                                    ("How do I top up my wallet?", "how_to")]))


if __name__ == "__main__":
    main()
