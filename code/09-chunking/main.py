"""Chunking: where you cut decides what search can find.

Replays visual/09-chunking.html. The runbook is built in code: a heading, a
reason, a table of 12 writers, and a 9-step procedure. The fake model lists
whatever writers the retrieved chunks hold. Run: python3 main.py
"""
import re

ROWS = [f"| writer-{i} | stop it |" for i in range(1, 13)]
RUNBOOK = "\n".join(
    ["# Lane reset", "Why: a reset stops every writer of the lane.", "",
     "## Writers to stop", "| name | action |", "|---|---|"] + ROWS +
    ["", "## Procedure"] + [f"{i}. step {i}" for i in range(1, 10)])
QUESTION = "which writers must stop for a lane reset"


# panel:start
def split_on_headings(text):
    chunks, path, cur = [], [], []
    for line in text.split("\n"):
        m = re.match(r"(#+) (.*)", line)                # @1 a heading starts a chunk
        if m:
            if cur:
                chunks.append("\n".join(cur))           # @1
            path = path[:len(m.group(1)) - 1] + [m.group(2)]
            cur = [" > ".join(path)]                    # @2 keep the heading path
        else:
            cur.append(line)                            # @3 tables stay whole
    chunks.append("\n".join(cur))
    return chunks


def split_fixed(text, cut_after_row=6):                 # @6 the bad splitter
    cut = text.index(ROWS[cut_after_row])               # @6 in a real one: every 500 tokens
    return [text[:cut], text[cut:]]                     # @6


def retrieve(chunks, question):                         # @4
    words = set(question.lower().split())               # @4
    score = lambda c: len(words & set(re.findall(r"[a-z]+", c.lower())))  # @4
    return max(chunks, key=score)                       # @4 the best match


def fake_model(chunks):                                 # @5 answers from these chunks only
    found = {w for c in chunks for w in re.findall(r"writer-\d+", c)}  # @5
    return sorted(found, key=lambda w: int(w[7:]))      # @5


def golden_check(chunks):                               # @7
    best = retrieve(chunks, QUESTION)                   # @7
    return all(r in best for r in ROWS)                 # @7 the right chunk, whole?
# panel:end


def main():
    print("-- tab 1 --")
    chunks = split_on_headings(RUNBOOK)
    print(len(chunks), "chunks:", [c.split("\n")[0] for c in chunks])
    best = retrieve(chunks, QUESTION)
    print("chunk 2 opens:", best.split("\n")[:2], "| table rows:", sum(r in best for r in ROWS))
    print("retrieved:", best.split("\n")[0], "| answer:", len(fake_model([best])), "of 12 writers")
    print("golden check:", golden_check(chunks))

    print("\n-- tab 2 --")
    chunks = split_fixed(RUNBOOK)
    print("chunk B starts:", chunks[1].split("\n")[0])
    best = retrieve(chunks, QUESTION)
    words = set(QUESTION.lower().split())
    for name, c in zip("AB", chunks):
        shared = sorted(words & set(re.findall(r"[a-z]+", c.lower())))
        print(f"chunk {name} shares {len(shared)}: {shared}")
    print("retrieved chunk A:", best == chunks[0], "| answer:", len(fake_model([best])), "of 12 writers")
    print("golden check:", golden_check(chunks))


if __name__ == "__main__":
    main()
