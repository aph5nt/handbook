"""Speculative decoding as a toy: a small draft model guesses K tokens,
the big target model checks them in one pass and keeps the agreed ones.

Standard library only. Both models are toys: the target "knows" the
answer it would write, the draft is a trigram counter trained on a
little text. The costs are example costs, counted in big-model steps.
"""
from collections import Counter, defaultdict

K = 4              # guesses per round
DRAFT_COST = 0.1   # one draft step costs a tenth of a big step (example)

DRAFT_TEXT = ("the swap moved 3 eth . the swap moved 3 eth . "
              "tokens to the pool . sent to 0x 9f 3c a1 . "
              'report : { "verdict" : " clean " , "risk" : " low " }').split()


def train_draft(words):
    """Trigram counts with a bigram fall-back: the cheap draft model."""
    tri, bi = defaultdict(Counter), defaultdict(Counter)
    padded = ["<s>", "<s>"] + words
    for a, b, c in zip(padded, padded[1:], padded[2:]):
        tri[(a, b)][c] += 1
        bi[b][c] += 1
    return tri, bi


def draft_guess(model, context, k):
    tri, bi = model
    out, ctx = [], list(context)
    for _ in range(k):
        a, b = (["<s>", "<s>"] + ctx)[-2:]
        table = tri.get((a, b)) or bi.get(b) or Counter({".": 1})
        word = table.most_common(1)[0][0]
        out.append(word)
        ctx.append(word)
    return out


def speculative(answer, model, before=(), show=False):
    """The target model writes `answer`. Count big passes and draft steps."""
    done, big, drafted = [], 0, 0
    while len(done) < len(answer):
        guesses = draft_guess(model, list(before) + done, K)
        drafted += K
        big += 1                                  # one pass checks all K
        target = answer[len(done):len(done) + K + 1]
        kept = 0
        while kept < len(guesses) and kept < len(target) and guesses[kept] == target[kept]:
            kept += 1                             # keep up to the first miss
        new = target[:kept + 1]                   # plus the big model's own token
        if show:
            print(f"  draft {guesses}  kept {kept}  big model adds "
                  f"{new[kept:] or ['(end)']}")
        done += new
    cost = big + drafted * DRAFT_COST
    return done, big, cost


def report(name, answer, model, before=()):
    print(f"{name}: {' '.join(answer)}")
    out, big, cost = speculative(answer, model, before, show=True)
    assert out == answer                          # same text as the big model alone
    speed = len(answer) / cost                    # plain decoding: 1 token per big step
    print(f"  {len(answer)} tokens, {big} big passes instead of {len(answer)}, "
          f"cost {cost:.1f} big steps, speed {speed:.2f}x")
    return len(answer), cost


model = train_draft(DRAFT_TEXT)
prose = "the swap moved tokens to the pool .".split()
hexa = "0x 4e 1b 77 c0 d2 e5 .".split()
js = '{ "verdict" : " clean " , "risk" : " low " }'.split()

runs = {"prose": report("prose", prose, model),
        "hex": report("hex", hexa, model, before=["sent", "to"]),
        "json": report("json", js, model, before=["report", ":"])}

# The fix: measure each kind of output, turn the draft off where it loses.
print("traffic: one prose, one hex and one JSON answer")
tokens = sum(n for n, _ in runs.values())
always = sum(c for _, c in runs.values())
measured = sum(min(c, n) for n, c in runs.values())   # off when slower than plain
print(f"  plain {tokens} tokens in {tokens} big steps: 1.00x")
print(f"  draft always on: {tokens / always:.2f}x")
print(f"  draft off for hex: {tokens / measured:.2f}x")
