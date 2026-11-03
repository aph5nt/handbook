"""HyDE: search with a fake answer.

Replays visual/09-hyde.html. Standard library only, no network.
The embedding is a real, tiny bag-of-words vector with cosine similarity.
The model is a fake that returns a fixed paragraph. Run: python3 main.py
"""
import math
import re
from collections import Counter

DOCS = {
    "lane C notes": "lane c notes: lane c is stuck? lane c restarts",
    "a stuck CI job": "ci job stuck: a b test job hangs",
    "lane reset runbook": "if lane b is stuck, check the graph sync lock, then stop the writers "
                          "and the readers of the lane before any reset, then note the heights of every table, "
                          "then restart the writers in order",
    "incident: lane B stall": "incident: the graph sync lock was held, the writers stalled, "
                              "check the lock first",
    "deploy scripts README": "deploy scripts: run the .sh script to clear and unstick a deploy",
    "CI job scripts": "ci scripts: run script.sh to clear a job",
}


def tokens(text):
    return re.findall(r"[a-z0-9_.\-]+", text.lower())


def embed(text):
    return Counter(tokens(text))


def cosine(a, b):
    dot = sum(a[t] * b[t] for t in a)
    return dot / (math.sqrt(sum(v * v for v in a.values())) * math.sqrt(sum(v * v for v in b.values())))


def vector_search(vec, top=2):
    return sorted(DOCS, key=lambda d: -cosine(vec, embed(DOCS[d])))[:top]


def keyword_search(text, top=2):
    q = set(tokens(text))
    scored = [(sum(1 for t in set(tokens(DOCS[d])) if t in q), d) for d in DOCS]
    return [d for s, d in sorted(scored, key=lambda x: -x[0]) if s > 0][:top]


def rrf(*lists):
    score = {}
    for hits in lists:
        for rank, doc in enumerate(hits, start=1):
            score[doc] = score.get(doc, 0) + 1 / (60 + rank)
    return sorted(score, key=lambda d: -score[d])


FAKES = {
    "good": "If lane B is stuck, check the graph sync lock and the writers.",
    "invented": "Run lane-b-unstick.sh to clear it, a script that will unstick the lane.",
}


def fake_model_write(question, kind):
    """Stands in for: 'write the paragraph a runbook would contain for: <question>'."""
    return FAKES[kind]


def fake_model_answer(question, docs):
    return f"answer built from '{docs[0]}'"


# panel:start
def hyde_search(question, kind="good", keywords_on_fake=False):
    vec_query = question                               # @1 direct embedding
    fake = None
    if len(question.split()) < 6:                      # @2 long questions embed well
        fake = fake_model_write(question, kind)        # @3
        vec_query = fake                               # @4 answer-shaped text
    vec_hits = vector_search(embed(vec_query))         # @5
    if keywords_on_fake and fake:                      # @9 wrong: the invented name steers it
        return rrf(keyword_search(fake), vec_hits), fake  # @9
    kw_hits = keyword_search(question)                 # @6 the user's own words only
    return rrf(kw_hits, vec_hits), fake                # @7 merge by rank


def answer(question, hits):
    return fake_model_answer(question, hits[:5])       # @8 the fake is never shown
# panel:end


def main():
    q = "lane B stuck?"
    print("question:", q)
    print("\n-- tab 1: search with the fake answer --")
    print("direct embedding:", vector_search(embed(q)))
    print(f"word count: {len(q.split())}, under 6, so HyDE runs")
    fake = fake_model_write(q, "good")
    print("fake answer:", fake)
    print("vector search with the fake:", vector_search(embed(fake)))
    print(answer(q, hyde_search(q)[0]), "(the fake is discarded)")

    print("\n-- tab 2: the fake answer invents a name --")
    bad = fake_model_write(q, "invented")
    print("fake answer:", bad)
    print("keyword search on the fake:", keyword_search(bad))
    print("vector search with the fake:", vector_search(embed(bad)))
    hits, _ = hyde_search(q, "invented", keywords_on_fake=True)
    print("merged, keywords on the fake:", hits[:2])
    print(answer(q, hits), "(the invented script name rides along)")
    print("fix: keywords on the real words, HyDE on vectors only")
    fixed, _ = hyde_search(q, "invented")
    qt = set(tokens(q))
    print("keywords on the real words:", [(d, sum(1 for t in set(tokens(DOCS[d])) if t in qt)) for d in keyword_search(q)],
          "(document, matching words)")
    kw, vec = keyword_search(q), vector_search(embed(bad))
    print(f"RRF: runbook 1/{60 + kw.index('lane reset runbook') + 1} (keyword rank), "
          f"deploy scripts README 1/{60 + vec.index('deploy scripts README') + 1} (vector rank), a tie: the runbook stays first")
    print("fused:", fixed[:2])
    long_q = "how do I stop the writers before a reset of lane B"
    print(f"long question ({len(long_q.split())} words): HyDE skipped, fake =", hyde_search(long_q)[1])


if __name__ == "__main__":
    main()
