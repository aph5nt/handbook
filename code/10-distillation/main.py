"""Distillation: replay the lesson "Distillation: a small student learns one task".

A teacher labels 10,000 prompts as exchange, mixer or unknown. A checker throws
out its wrong labels. A small student learns from the checked pairs and is
compared with the teacher on a 100-case golden set. A router sends only label
requests to the student. Toy data, standard library only. Run: python3 main.py
"""
from collections import Counter, defaultdict


def truth(bucket):
    """The real kind of an address in feature bucket 0..19."""
    if bucket >= 17:
        return "mixer"
    if bucket <= 6:
        return "exchange"
    return "unknown"


def teacher(bucket, slot):
    """The big model: right except on 3 of 5 slots of bucket 19, where it says exchange."""
    if bucket == 19 and slot < 3:
        return "exchange"
    return truth(bucket)


def student_feature(bucket):
    """The small model cannot tell buckets 5, 6 and 7 apart."""
    return 5 if bucket in (5, 6, 7) else bucket


def train_student(pairs):
    """Majority label per feature the student can see."""
    votes = defaultdict(Counter)
    for bucket, label in pairs:
        votes[student_feature(bucket)][label] += 1
    return {f: c.most_common(1)[0][0] for f, c in votes.items()}


def route(task, student, bucket):
    """Narrow task to the student, everything else to the teacher."""
    if task != "label":
        return "teacher"
    return student[student_feature(bucket)]


prompts = [(i % 20, (i // 20) % 5) for i in range(10_000)]   # (bucket, slot)
labelled = [(b, teacher(b, s)) for b, s in prompts]
checked = [(b, lab) for b, lab in labelled if lab == truth(b)]  # the checker
print(f"teacher labels: {len(labelled):,}, thrown out by the checker: {len(labelled) - len(checked)}")

student = train_student(checked)
copycat = train_student(labelled)                  # no filter: learns the mistakes

golden = [(k % 20, (k // 20) % 5) for k in range(100)]


def score(label_fn):
    return sum(label_fn(b, s) == truth(b) for b, s in golden)


print(f"golden set: teacher {score(teacher)}%, student {score(lambda b, s: student[student_feature(b)])}%")
print(f"student trained without the filter: {score(lambda b, s: copycat[student_feature(b)])}%")
print("  bucket 19 (a mixer) is labelled", copycat[19], "by that student")

print("router: label request ->", route("label", student, 2))
print("router: explain request ->", route("explain", student, 2))
