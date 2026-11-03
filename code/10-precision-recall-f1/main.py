"""Precision, recall and F1: replay the lesson about the mixer detector.

1,000 addresses, 50 real mixers. Each address gets a detector score; a
threshold turns the score into a flag. Standard library only.
Run: python3 main.py
"""

# (score, is_mixer, how many such addresses) - an example golden set
GOLDEN = [(0.95, True, 20), (0.75, True, 10), (0.45, True, 15), (0.10, True, 5),
          (0.75, False, 10), (0.45, False, 65), (0.10, False, 875)]

def confusion(threshold):
    """Count true positives, false positives, false negatives, true negatives."""
    tp = fp = fn = tn = 0
    for score, mixer, n in GOLDEN:
        flagged = score >= threshold
        if flagged and mixer: tp += n
        elif flagged: fp += n
        elif mixer: fn += n
        else: tn += n
    return tp, fp, fn, tn

def scores(threshold):
    tp, fp, fn, tn = confusion(threshold)
    precision = tp / (tp + fp) if tp + fp else None   # no flags: undefined
    recall = tp / (tp + fn)
    f1 = 2 * precision * recall / (precision + recall) if precision else 0.0
    accuracy = (tp + tn) / (tp + fp + fn + tn)
    return tp, fp, fn, precision, recall, f1, accuracy

def show(name, threshold):
    tp, fp, fn, p, r, f1, acc = scores(threshold)
    ptxt = f"{p:.0%}" if p is not None else "no flags"
    print(f"{name:7} threshold {threshold}: flags {tp + fp}, TP {tp}, FP {fp}, FN {fn}, "
          f"precision {ptxt}, recall {r:.0%}, F1 {f1:.0%}, accuracy {acc:.0%}")

print("Tab 1 and tab 2")
show("middle", 0.7)     # the detector in the story: 40 flags
show("nothing", 1.1)    # flags nothing at all
show("high", 0.9)       # only the surest cases
show("low", 0.4)        # anything a little odd
