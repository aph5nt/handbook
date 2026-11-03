"""Prompt, RAG or fine-tune: replay the lesson "where the change lives".

Tab 1 routes three needs down the decision tree and sizes a LoRA adapter.
Tab 2 trains facts into "weights" on Monday, renames a table on Wednesday,
and scores a golden set before and after moving the facts to retrieval.
Standard library only. Run: python3 main.py
"""


def route(need):
    """The decision tree, in the order the lesson walks it."""
    if need["is_rule"] or need["is_format_example"]:
        return "prompt"                  # a skill in git: minutes, reversible
    if need["changes_often"] or need["is_private"]:
        return "rag"                     # fetch it at run time
    if need["examples"] >= 1000 and need["fixed_shape"]:
        return "fine-tune (LoRA)"        # shape and style, never facts
    return "prompt"


def need(name, **kw):
    base = dict(is_rule=False, is_format_example=False, changes_often=False,
                is_private=False, examples=0, fixed_shape=False)
    base.update(kw)
    return name, base


def lora_size(d, rank):
    """Trainable numbers for one d x d weight: full matrix versus A (d x r) plus B (r x d)."""
    return d * d, 2 * d * rank


print("Tab 1: route three needs")
for name, n in [need("answer rules", is_rule=True),
                need("table schemas", changes_often=True),
                need("fixed JSON report", examples=5000, fixed_shape=True)]:
    print(f"  {name:18} -> {route(n)}")
full, lora = lora_size(4096, 16)
print(f"LoRA on one 4096 x 4096 weight, rank 16: {lora:,} of {full:,} numbers = {lora / full:.1%}")
print(f"  at 2 bytes each: adapter part {lora * 2 / 1e6:.2f} MB, full matrix {full * 2 / 1e6:.1f} MB")

print()
print("Tab 2: facts in the weights")
TOPICS = [f"topic {i}" for i in range(50)]
monday = {t: f"robinhood.table_{i}" for i, t in enumerate(TOPICS)}
monday["topic 0"] = "robinhood.dex_swaps"           # the swaps table, Monday
weights = dict(monday)                               # fine-tuned: frozen copy
wednesday = dict(monday)
renamed = ["topic 0", "topic 7", "topic 13", "topic 21", "topic 34", "topic 42"]
for t in renamed:
    wednesday[t] = wednesday[t].replace("robinhood.", "robinhood.core_")
print("where do swaps live? weights say:", weights["topic 0"], "| today:", wednesday["topic 0"])

not_retrieved = {"topic 5", "topic 18", "topic 40"}  # retrieval misses, measured


def answer_from_weights(topic):
    return weights[topic]                            # Monday's name, forever


def answer_from_rag(topic):
    if topic in not_retrieved:
        return "not found"                           # the chunk was not in the top results
    return wednesday[topic]                          # fetched today, with its source


golden = {t: wednesday[t] for t in TOPICS}           # the right answers today
before = sum(answer_from_weights(t) == golden[t] for t in TOPICS)
after = sum(answer_from_rag(t) == golden[t] for t in TOPICS)
print(f"golden set before (facts in weights): {before} of 50 = {before / 50:.0%}")
print(f"golden set after (facts through RAG): {after} of 50 = {after / 50:.0%}")
