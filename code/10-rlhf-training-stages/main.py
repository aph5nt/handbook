"""Pre-training, instruction tuning and RLHF: replay the lesson "From base model to assistant".

A toy at a tiny scale, standard library only:
  stage 1, pre-training: a trigram model learns next words from web-like text
  stage 2, instruction tuning: question and answer pairs are added
  stage 3, preference tuning: a reward model learns from people's picks,
           and the policy answers with the highest-reward candidate
Real models do the same three stages with billions of weights. Run: python3 main.py
"""
from collections import Counter, defaultdict


def train_next_word(texts, model=None, weight=1):
    """Count which word follows each pair of words: next-token prediction."""
    model = model if model is not None else defaultdict(Counter)
    for t in texts:
        words = t.split()
        for a, b, c in zip(words, words[1:], words[2:]):
            model[(a, b)][c] += weight
    return model


def generate(model, prompt, max_words=12):
    """Greedy: always take the most frequent next word."""
    words = prompt.split()
    for _ in range(max_words):
        nxt = model.get((words[-2], words[-1]))
        if not nxt:
            break
        w = nxt.most_common(1)[0][0]
        words.append(w)
        if w == ".":
            break
    return " ".join(words[len(prompt.split()):])


WEB = ["who funded this wallet ? who sent it ? when ? which exchange ?",
       "faq : who sent it ? when ? which exchange ?"]
SFT = ["who funded this wallet ? it was funded by two wallets , then an exchange ."]

FEATURES = ("confident", "says_need_data", "long")


def reward(w, answer):
    return sum(w[f] * answer[f] for f in FEATURES)


def train_reward(picks, epochs=10):
    """Perceptron on pairs: push the winner's reward above the loser's."""
    w = {f: 0.0 for f in FEATURES}
    for _ in range(epochs):
        for win, lose in picks:
            if reward(w, win) <= reward(w, lose):
                for f in FEATURES:
                    w[f] += win[f] - lose[f]
    return w


def policy(w, candidates, data_has_funder):
    """Pick the highest-reward answer. The fix adds a fact check in code first."""
    best = max(candidates, key=lambda a: reward(w, a))
    return best["text"]


def checked_policy(w, candidates, data_has_funder):
    allowed = [a for a in candidates if data_has_funder or not a["claims_funder"]]
    return max(allowed, key=lambda a: reward(w, a))["text"]


GUESS = {"text": "Funded by Binance hot wallet, certainly.", "confident": 1, "says_need_data": 0, "long": 1, "claims_funder": True}
HONEST = {"text": "The data shows no funder. Give me another address?", "confident": 0, "says_need_data": 1, "long": 0, "claims_funder": False}

q = "who funded this wallet ?"
base = train_next_word(WEB)
print("stage 1, base model:", generate(base, q))
tuned = train_next_word(SFT, train_next_word(WEB), weight=3)
print("stage 2, instruction tuned:", generate(tuned, q))

good_picks = [(HONEST, GUESS)] * 8          # people pick the honest answer
w = train_reward(good_picks)
print("stage 3, reward weights from good picks:", w)
print("   assistant answers:", policy(w, [GUESS, HONEST], data_has_funder=False))

bad_picks = [(GUESS, HONEST)] * 8           # judges cannot check, pick the confident one
w_bad = train_reward(bad_picks)
print("failure, reward weights from confident picks:", w_bad)
print("   assistant answers:", policy(w_bad, [GUESS, HONEST], data_has_funder=False))
print("fix, fact check in code first:", checked_policy(w_bad, [GUESS, HONEST], data_has_funder=False))
