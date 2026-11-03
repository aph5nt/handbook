"""GraphRAG walk: follow facts from one anchor.

Replays visual/09-graphrag.html. Links are (from, kind, to, source). The walk
is real. The fake model states whatever rows it gets. Run: python3 main.py
"""
import time

CHAIN = [
    ("creator", "created", "3,000 agent tokens", "chain"),
    ("creator", "funded by", "Funder A", "chain"), ("creator", "funded by", "Funder B", "chain"),
    ("Funder A", "withdrew from", "Exchange hot wallet", "chain"), ("Funder B", "withdrew from", "Exchange hot wallet", "chain"),
    ("Funder A", "swapped on", "DEX pool", "chain"), ("Funder B", "sent to", "other wallets", "chain"),
]
EXTRACTED = [("creator", "owned by", "Wallet Z", "model"), ("Wallet Z", "belongs to", "Exchange Y", "model")]
FOLLOW = {"created", "funded by", "withdrew from", "owned by", "belongs to"}
TEXT_CHUNKS = ["agent tokens launch", "agent tokens fees", "agent tokens rules", "agent tokens news", "agent tokens faq"]


def fake_model(rows):
    if any(r[2] == "Exchange Y" for r in rows):
        return "Behind the creator is Exchange Y."
    return "Both funders of the creator wallet withdrew from one exchange hot wallet."


# panel:start
HOP_CAP = 2                                          # @5
TIME_LIMIT_S = 10                                    # @5


def text_search(question):                           # @1 chunks hold words, no links
    return [c for c in TEXT_CHUNKS if "agent tokens" in question][:5]  # @1


def walk(graph, anchor, trusted=("chain",)):         # @2 start from the known entity
    rows, frontier = [], {anchor}                    # @2
    deadline = time.monotonic() + TIME_LIMIT_S       # @5
    for hop in range(HOP_CAP):                       # @3,5 stops at the hop cap
        if time.monotonic() > deadline:              # @5 the time limit
            break                                    # @5
        next_frontier = set()                        # @3
        for src, kind, dst, source in graph:         # @3
            if src in frontier and kind in FOLLOW:   # @4 only the links that answer
                if source not in trusted:            # @7 a link must be a record
                    continue                         # @7
                rows.append((src, kind, dst))        # @4
                next_frontier.add(dst)               # @3
        frontier = next_frontier                     # @3
    return rows                                      # @5


def graph_answer(graph, anchor, **kw):
    rows = walk(graph, anchor, **kw)                 # @6 connected facts, not chunks
    return rows, fake_model(rows)                    # @6


def score_extraction(extracted, golden):             # @8
    hits = sum(1 for l in extracted if l[:3] in golden)  # @8
    return hits / len(extracted)                     # @8 score before anyone walks it
# panel:end


def main():
    print("-- tab 1 --")
    print("text search:", len(text_search("who is behind the wallet that made 3,000 agent tokens")), "chunks, funders named: 0")
    rows, ans = graph_answer(CHAIN, "creator")
    for r in rows:
        print(" ", r)
    print(f"  {len(rows)} rows. answer: {ans}")
    skipped = [f"{d} ({k}: wrong link kind)" for s, k, d, _ in CHAIN if k not in FOLLOW]
    print(f"  not followed: {', '.join(skipped)}. stopped: {HOP_CAP} hops, limit {TIME_LIMIT_S} s")

    print("\n-- tab 2 --")
    rows, ans = graph_answer(EXTRACTED, "creator", trusted=("chain", "model"))
    print("  walk over guesses:", rows)
    print("  answer:", ans)
    rows, ans = graph_answer(CHAIN + EXTRACTED, "creator")
    print(f"  records only: {len(rows)} rows, answer: {ans}")
    print("  extraction score:", score_extraction(EXTRACTED, {("creator", "owned by", "Wallet Q")}))


if __name__ == "__main__":
    main()
