"""Map-reduce: the same prompt on every piece, then one call combines.

Replays visual/09-map-reduce.html. fake_model is a rule over a made-up month
of 30 notes. Note 28 is the only severity 1. Run: python3 main.py
"""
from concurrent.futures import ThreadPoolExecutor

WINDOW_WORDS = 14_400  # example: what one reduce call holds well
NOTES = [{"id": i, "sev": 1 if i == 28 else 2 if i % 2 else 3,
          "service": "graph sync" if i == 28 else "lane B" if i % 2 else "proxy",
          "cause": "lock" if i == 28 else "disk full" if i % 2 else "timeout"} for i in range(1, 31)]


def fake_map(note, schema):
    if schema:
        return {"note": note["id"], "sev": note["sev"], "service": note["service"],
                "cause": note["cause"], "words": 20}
    return {"note": note["id"], "sev": note["sev"], "service": note["service"], "words": 600}


# panel:start
def map_all(notes, schema=True):
    with ThreadPoolExecutor(max_workers=30) as pool:   # @1 all at the same time
        return list(pool.map(lambda n: fake_map(n, schema), notes))  # @2 same prompt


def fit_window(rows):
    kept, words = [], 0
    for row in rows:                                   # @4
        if words + row["words"] > WINDOW_WORDS:        # @4 the harness cuts, no error
            break                                      # @4
        kept.append(row)
        words += row["words"]
    return kept


def reduce_rows(rows):
    return sorted(rows, key=lambda r: r["sev"])        # @5 worst first


def reduce_in_groups(rows, size=6):
    groups = [rows[k:k + size] for k in range(0, len(rows), size)]  # @6
    results = [reduce_rows(g) for g in groups]         # @6 5 group results
    return reduce_rows([r for g in results for r in g]), len(groups)  # @6


def check_counted(report, notes):                      # @7
    return len(report) == len(notes)                   # @7 every piece counted
# panel:end


def main():
    print("-- tab 1 --")
    rows = map_all(NOTES)
    report = reduce_rows(fit_window(rows))
    for r in (rows[0], rows[1], rows[27]):
        print(f"note {r['note']}: sev {r['sev']}, {r['service']}, {r['cause']}, {r['words']} words")
    print(f"{len(rows)} map calls + 1 reduce, {sum(r['words'] for r in rows)} words into reduce")
    print(f"report: {len(report)} counted, worst first: note {report[0]['note']} sev {report[0]['sev']}")

    print("\n-- tab 2 --")
    rows = map_all(NOTES, schema=False)
    kept = fit_window(rows)
    print(f"prose: {sum(r['words'] for r in rows)} words, window {WINDOW_WORDS}")
    print(f"{len(kept)} answers fit")
    print(f"cut with no error: notes {kept[-1]['note'] + 1} to 30")
    report = reduce_rows(kept)
    print("note 28 in the report:", any(r["note"] == 28 for r in report), "| all counted:", check_counted(report, NOTES),
          f"({len(report)} rows, {len(NOTES)} notes)")
    report, groups = reduce_in_groups(map_all(NOTES))
    print(f"fix: {groups} groups of 6, all counted: {check_counted(report, NOTES)}, top: note {report[0]['note']}")


if __name__ == "__main__":
    main()
