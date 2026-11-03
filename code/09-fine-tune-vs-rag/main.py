"""Prompt, retrieve, or fine-tune?

Replays visual/09-fine-tune-vs-rag.html. Standard library only, no network.
There is no model to train here. The runnable core is the decision rule over
the lesson's three needs, and a week-later replay that compares a fine-tuned
snapshot with a lookup that reads the registry at question time.
Run: python3 main.py
"""
from dataclasses import dataclass

REGISTRY_ON_TRAINING_DAY = {"pool 0x41..aa01": "Exchange A", "pool 0x41..aa02": "Exchange B",
                            "pool 0x41..aa03": "Exchange C"}
REGISTRY_A_WEEK_LATER = dict(REGISTRY_ON_TRAINING_DAY, **{"pool 0x41..aa04": "Exchange D",
                                                         "pool 0x41..aa05": "Exchange E"})


@dataclass
class Need:
    name: str
    facts_change: bool = False
    needs_source: bool = False
    format_or_tone: bool = False
    narrow: bool = False
    high_volume: bool = False
    has_labels_and_eval: bool = False


# panel:start
def choose(need):                                           # @1
    if need.facts_change or need.needs_source:              # @2 facts
        return "retrieval or a lookup tool"                 # @3 read fresh at question time
    if need.format_or_tone:                                 # @4
        return "prompt with a few examples"                 # @5 changed in minutes
    if need.narrow and need.high_volume and need.has_labels_and_eval:  # @6
        return "fine-tune a small model"                    # @7
    return "prompt first"                                   # @8 the cheapest first


def lookup(pool, registry_file):                            # @9
    name = registry_file.get(pool)                          # @9 read the file now
    return (name, "registry file") if name else (None, "not in the registry")  # @9


class FineTunedSnapshot:                                    # @10 wrong for facts
    def __init__(self, registry):                           # @10
        self.weights = dict(registry)                       # @10 frozen on training day

    def answer(self, pool):                                 # @10
        return self.weights.get(pool), None                 # @10 no source to cite
# panel:end


def main():
    needs = [Need("facts that change weekly: new exchange pools", facts_change=True, needs_source=True),
             Need("output layout and tone: the risk report", format_or_tone=True),
             Need("one narrow task, huge volume: label transfers", narrow=True, high_volume=True,
                  has_labels_and_eval=True)]
    print("-- tab 1: each need, its tool --")
    for n in needs:
        print(f"{n.name} -> {choose(n)}")

    print("\n-- tab 2: facts baked into a fine-tune --")
    model = FineTunedSnapshot(REGISTRY_ON_TRAINING_DAY)
    print(f"training day: {len(model.weights)} pools trained into the weights")
    new = [p for p in REGISTRY_A_WEEK_LATER if p not in REGISTRY_ON_TRAINING_DAY]
    print(f"a week later: {len(REGISTRY_A_WEEK_LATER)} pools in the registry, {len(new)} new")
    for p in new:
        print(f"  fine-tuned model on {p}: {model.answer(p)}")
    old = list(REGISTRY_ON_TRAINING_DAY)
    print(f"  the {len(old)} old pools, stated as fact:", [model.answer(p)[0] for p in old])
    print("  source for", "pool 0x41..aa01", "->", model.answer("pool 0x41..aa01")[1])
    print("fix: lookup at question time")
    for p in new:
        print(f"  lookup {p}: {lookup(p, REGISTRY_A_WEEK_LATER)}")


if __name__ == "__main__":
    main()
