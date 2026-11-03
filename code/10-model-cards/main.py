"""Model cards: replay the lesson "Model cards: read the limits before production finds them".

A model card as data, a fit check against one task, then the golden set.
The golden results are the lesson's example, not a measurement.
Standard library only, no network. Run: python3 main.py
"""

THRESHOLD = 0.90   # share of golden cases that must keep every fact

card = {
    "intended_use": ["general text", "chat", "summaries"],
    "weak_languages": ["Polish"],
    "weak_at": ["exact arithmetic"],
    "benchmark_language": "English",
    "cutoff_years_ago": 2,
    "safety_tests": ["refusals"],
}
task = {"use": "summaries", "language": "Polish", "needs_recent_facts": True}

# 20 Polish golden cases: True means the summary kept every fact.
golden = [True] * 13 + [False] * 7


def fit_check(card, task, read_limits=True):
    """Return (ok, notes). A team that skips the limits reads only the use and the high scores."""
    notes = []
    if task["use"] not in card["intended_use"]:
        return False, ["not an intended use"]
    if read_limits:
        if task["language"] in card["weak_languages"]:
            notes.append(f"warning: {task['language']} is a weak language")
        if task["needs_recent_facts"] and card["cutoff_years_ago"] >= 1:
            notes.append(f"cutoff {card['cutoff_years_ago']} years ago: fetch recent facts through RAG")
        if card["benchmark_language"] != task["language"]:
            notes.append(f"benchmarks are {card['benchmark_language']}: not evidence for this task")
    return True, notes


def golden_verdict(results):
    passed = sum(results)
    share = passed / len(results)
    verdict = "pilot" if share >= THRESHOLD else "pick another model"
    return f"{passed} of {len(results)} keep every fact = {share:.0%}: {verdict}"


print("Tab 1: read the card, then confirm")
ok, notes = fit_check(card, task)
print("intended use fits:", ok)
for n in notes:
    print("  ", n)
print("golden set:", golden_verdict(golden))

print()
print("Tab 2: the team that trusted the score")
ok, notes = fit_check(card, task, read_limits=False)
print("intended use fits:", ok, "| notes read:", notes or "none")
print("golden set: not run, straight to production")
print("production: 7 in 20 summaries drop a fact (the same cases the golden set would have shown)")
