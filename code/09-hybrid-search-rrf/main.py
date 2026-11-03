"""Hybrid search and reciprocal rank fusion (RRF).

Replays visual/09-hybrid-search-rrf.html. Standard library only, no network.
The two searches are fakes that return the lesson's ranked lists with their
raw scores. The fusion is real: 1 / (60 + rank), summed over both lists.
Run: python3 main.py
"""

QUESTION = "How do I reset core_dex_swaps on lane B?"


def bm25_search(question):
    """Fake keyword index: BM25 scores grow with rare-word matches, no fixed top."""
    return [("core_dex_swaps doc", 12.4), ("lane reset runbook", 9.1),
            ("swap sync note", 4.0), ("price oracle doc", 1.2)]


def vector_search(question):
    """Fake vector index: cosine similarity, always below 1."""
    return [("lane reset runbook", 0.83), ("incident: lane B stall", 0.80),
            ("swap sync note", 0.71), ("core_dex_swaps doc", 0.62)]


# panel:start
K = 60                                                   # @6 the RRF constant


def rrf(*lists):                                         # @3
    score = {}                                           # @3
    for hits in lists:                                   # @4
        for rank, (doc, _raw) in enumerate(hits, start=1):  # @5 raw score ignored
            score[doc] = score.get(doc, 0) + 1 / (K + rank)  # @6 only the position counts
    return sorted(score.items(), key=lambda kv: -kv[1])  # @7


def raw_merge(*lists):                                   # @9 the wrong merge
    best = {}                                            # @9
    for hits in lists:                                   # @9
        for doc, raw in hits:                            # @9
            best[doc] = max(best.get(doc, 0), raw)       # @9 12.4 beats 0.83 every time
    return sorted(best.items(), key=lambda kv: -kv[1])   # @9


def hybrid_search(question, top=50):
    keyword = bm25_search(question)[:top]                # @1
    vector = vector_search(question)[:top]               # @2
    return rrf(keyword, vector)[:top]                    # @8 fused list to the re-ranker
# panel:end


def show(title, rows, places=2):
    print(title)
    for n, (doc, s) in enumerate(rows, start=1):
        print(f"  {n}. {doc}  ({s:.{places}f})" if s < 1 else f"  {n}. {doc}  ({s})")


def main():
    print("question:", QUESTION)
    print("\n-- tab 1: merge by rank (RRF) --")
    show("BM25 list:", bm25_search(QUESTION))
    show("vector list:", vector_search(QUESTION))
    runbook_bm25, runbook_cos = 9.1, 0.83
    print(f"raw sum for the runbook: {runbook_bm25} + {runbook_cos} = {runbook_bm25 + runbook_cos:.2f}")
    fused = hybrid_search(QUESTION)
    lists = [bm25_search(QUESTION), vector_search(QUESTION)]
    for doc, s in fused:
        parts = [f"1/{K + [d for d, _ in hits].index(doc) + 1}" for hits in lists if doc in [d for d, _ in hits]]
        print(f"  {doc}: {' + '.join(parts)} = {s:.4f}")
    show("fused by RRF, sum of 1/(60+rank):", fused, places=4)
    print(f"fused list to the re-ranker: {len(fused)} documents (cut at top=50)")
    top4 = [d for d, _ in fused[:4]]
    print("fused top 4:", top4)

    print("\n-- tab 2: merge by raw score --")
    merged = raw_merge(bm25_search(QUESTION), vector_search(QUESTION))
    show("sorted by raw score:", merged)
    rank = [d for d, _ in merged].index("incident: lane B stall") + 1
    print(f"incident note: rank {rank}, below every keyword hit")
    outlier, low, runbook = 40.0, 1.2, 9.1
    print(f"min-max with an outlier of {outlier}: runbook = ({runbook} - {low}) / ({outlier} - {low}) = "
          f"{(runbook - low) / (outlier - low):.2f}")
    print("fix: RRF again ->", [d for d, _ in rrf(bm25_search(QUESTION), vector_search(QUESTION))[:4]])


if __name__ == "__main__":
    main()
