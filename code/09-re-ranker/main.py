"""Re-ranker: a slower, sharper second look.

Replays visual/09-re-ranker.html. Standard library only, no network.
The first stage is a fake that returns 50 candidates with the lane reset
runbook at rank 9. The cross-encoder is a fake model: a fixed relevance table
for the question "How do I reset lane B?". Run: python3 main.py
"""

QUESTION = "How do I reset lane B?"
SHOWN = ["incident: lane B stall", "price oracle doc", "swap sync note", "core_dex_swaps doc",
         "graph sync lock notes", "deploy checklist", "indexer README", "alert routing notes",
         "lane reset runbook"]
# what a cross-encoder says after reading the question and the chunk together
RELEVANCE = {"lane reset runbook": 0.97, "incident: lane B stall": 0.81, "graph sync lock notes": 0.74,
             "swap sync note": 0.66, "core_dex_swaps doc": 0.58, "price oracle doc": 0.12,
             "deploy checklist": 0.09, "indexer README": 0.05, "alert routing notes": 0.04}
INDEX_SIZE = 200_000  # chunks in the whole index (example)
pair_calls = 0


def first_stage(question, top=50):
    """Fake hybrid search: fast, rough, 50 candidates."""
    filler = [f"other chunk {n}" for n in range(top - len(SHOWN))]
    return (SHOWN + filler)[:top]


def cross_encoder(question, doc):
    """Fake model: one slow pass over the pair (question, doc)."""
    global pair_calls
    pair_calls += 1
    return RELEVANCE.get(doc, 0.01)


def fake_answer(question, context):
    if context[0] == "lane reset runbook":
        return "the runbook steps, cited: [lane reset runbook]"
    return f"steps guessed from '{context[0]}' (half the writers missed)"


# panel:start
def answer(question, rerank=True):
    candidates = first_stage(question, top=50)         # @1 wide and cheap
    if not rerank:                                     # @7 no second look
        return candidates[:5], fake_answer(question, candidates[:5])  # @7
    score = {}
    for doc in candidates:                             # @2
        score[doc] = cross_encoder(question, doc)      # @3 reads both together
    ranked = sorted(candidates, key=lambda d: -score[d])  # @4
    context = ranked[:5]                               # @5 narrow and sharp
    return context, fake_answer(question, context)     # @6 cites the chunks


def rerank_whole_index(question):                      # @8 the opposite mistake
    return INDEX_SIZE                                  # @8 one slow pass per chunk
# panel:end


def main():
    global pair_calls
    print("question:", QUESTION)
    cands = first_stage(QUESTION)
    print("\n-- tab 1: re-rank the top 50 --")
    print(f"first stage: {len(cands)} candidates, runbook at rank {cands.index('lane reset runbook') + 1}")
    ctx, ans = answer(QUESTION)
    print(f"cross-encoder pairs scored: {pair_calls}")
    for n, d in enumerate(ctx, start=1):
        print(f"  prompt {n}. {d}  ({RELEVANCE[d]})")
    print("answer:", ans)
    print("fall away: price oracle doc", RELEVANCE["price oracle doc"], "deploy checklist", RELEVANCE["deploy checklist"])

    print("\n-- tab 2: no re-ranker --")
    ctx, ans = answer(QUESTION, rerank=False)
    print("prompt:", ctx)
    print("runbook in the prompt:", "lane reset runbook" in ctx)
    print("answer:", ans)
    pairs = rerank_whole_index(QUESTION)
    print(f"re-rank the whole index: {pairs:,} pairs per question, against 50 "
          f"(example: at 20 ms a pair, {pairs * 0.02 / 60:.0f} minutes against 1 second)")


if __name__ == "__main__":
    main()
