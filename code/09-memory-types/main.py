"""Memory types: buffer, summary, vector, entity.

Replays visual/09-memory-types.html. The fake model answers from whatever
the prompt holds. The vector store is a small bag-of-words cosine.
Run: python3 main.py
"""
import math
import re
from collections import Counter

ADDRESS = "0x9f2c4e7a1b3d5f6e8a0c2b4d6f8e0a1c3b5d0071"
OLD_TURNS = [
    f"Monday: traced wallet {ADDRESS}, funded by a mixer",
    "Monday: the report format is a table",
    "Tuesday: the swap pool list was refreshed",
]
QUESTION = "Did that wallet from Monday reach an exchange?"


def vec(text):
    return Counter(re.findall(r"[a-z0-9]+", text.lower()))


def cosine(a, b):
    top = sum(a[w] * b[w] for w in a)
    return top / (math.sqrt(sum(v * v for v in a.values())) * math.sqrt(sum(v * v for v in b.values())) or 1)


def fake_model(prompt):
    if "sanctioned mixer" in prompt:
        return "That wallet is a sanctioned mixer: do not clear it."
    if ADDRESS in prompt:
        return "Yes: about 2 days later, it reached one exchange."
    return "No known risk."


# panel:start
class Memory:
    def __init__(self):
        self.buffer = []                                  # @1 last 6 turns, word for word
        self.summary = ""                                 # @2 gist of older turns
        self.vectors = list(OLD_TURNS)                    # @3 old turns, searchable
        self.entities = {}                                # @4 pinned facts per name

    def add_turn(self, turn):
        self.buffer.append(turn)                          # @1
        if len(self.buffer) > 6:                          # @2
            old = self.buffer.pop(0)                      # @2
            self.summary = " ".join((self.summary + " " + old).split()[-12:])  # @2 keeps 12 words: each rewrite loses a little

    def pin(self, name, fact):                            # @7
        self.entities[name] = fact                        # @7 no summary touches this

    def recall(self, question):                           # @3
        q = vec(question)                                 # @3
        return max(self.vectors, key=lambda t: cosine(q, vec(t)))  # @3 best match

    def prompt(self, question, use_vector=True, use_entity=True):
        recalled = self.recall(question) if use_vector else ""  # @3
        names = [n for n in self.entities if n in recalled]     # @4
        facts = [self.entities[n] for n in names] if use_entity else []  # @4
        parts = facts + [self.summary, recalled] + self.buffer + [question]  # @5
        return "\n".join(p for p in parts if p)           # @5
# panel:end


def main():
    print("-- tab 1: four memories fill one prompt --")
    m = Memory()
    m.summary = "traced a mixer-funded wallet on Monday"
    m.buffer = ["today: checked the pool list", "today: report format ok"]
    m.pin(ADDRESS, f"{ADDRESS}: funded by a mixer, label added Monday")
    print("buffer:", len(m.buffer), "turns, summary:", m.summary)
    print("vector recall:", m.recall(QUESTION))
    print("entity fact:", m.entities[ADDRESS])
    print("answer:", fake_model(m.prompt(QUESTION)))

    print("\n-- tab 2: summary drops a fact --")
    m = Memory()
    m.add_turn(f"Monday: {ADDRESS} is a sanctioned mixer")
    for day in range(10):
        m.add_turn("checked some wallets")
    print("turns added: 11, buffer keeps", len(m.buffer), "| summary keeps the last 12 words")
    print("summary after rewrites:", repr(m.summary.strip()))
    print("address still in the summary:", ADDRESS in m.summary, "| 'sanctioned' still in it:", "sanctioned" in m.summary)
    print("answer:", fake_model(m.prompt(QUESTION, use_vector=False, use_entity=False)))
    m.vectors = list(OLD_TURNS)
    m.pin(ADDRESS, f"{ADDRESS} is a sanctioned mixer")
    print("pinned:", m.entities[ADDRESS])
    print("answer:", fake_model(m.prompt(QUESTION)))


if __name__ == "__main__":
    main()
