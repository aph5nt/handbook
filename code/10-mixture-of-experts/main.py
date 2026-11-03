"""Mixture of experts as a toy: a router scores eight experts per token,
the top two compute, their outputs are mixed by the router's weights.

Standard library only. The router scores, the experts and the sizes are
toy numbers chosen for the lesson, not taken from a real model.
"""
import math
import random

EXPERTS, TOP = 8, 2


def softmax(xs):
    m = max(xs)
    e = [math.exp(x - m) for x in xs]
    return [v / sum(e) for v in e]


# The learned router, as one logit per expert for each token (toy values).
ROUTER = {
    "swap": [math.log(p) for p in [.04, .48, .03, .03, .32, .02, .05, .03]],
    "pool": [math.log(p) for p in [.41, .05, .04, .06, .03, .04, .33, .04]],
}


def expert(k, x):
    """Expert k: a tiny feed-forward block of its own (toy weights)."""
    hidden = [max(0.0, (k + 1) * v - 0.5) for v in x]   # layer, then ReLU
    return [round(0.1 * h, 3) for h in hidden]


def route(logits, bias=None):
    scores = softmax(logits)
    pick_by = [s + (bias[k] if bias else 0) for k, s in enumerate(scores)]
    top = sorted(range(EXPERTS), key=lambda k: -pick_by[k])[:TOP]
    total = sum(scores[k] for k in top)
    return scores, top, [scores[k] / total for k in top]   # weights sum to 1


def moe_layer(token, x):
    scores, top, weights = route(ROUTER[token])
    print(f"{token!r}: scores {[round(s, 2) for s in scores]}")
    print(f"  top {TOP}: experts {[k + 1 for k in top]}, "
          f"weights {[round(w, 2) for w in weights]}")
    outs = [expert(k, x) for k in top]           # only these two compute
    mixed = [sum(w * o[i] for w, o in zip(weights, outs)) for i in range(len(x))]
    print(f"  output {[round(v, 3) for v in mixed]}, "
          f"{EXPERTS - TOP} experts idle for this token")


x = [1.0, 0.5, 2.0]
moe_layer("swap", x)
moe_layer("pool", x)

# Total versus active, with example sizes at 16 bits.
EXPERT_GB, SHARED_GB, KV_GB = 5, 3, 8
active = SHARED_GB + TOP * EXPERT_GB
total = SHARED_GB + EXPERTS * EXPERT_GB
print(f"active per token: {active} GB of weights, total: {total} GB")
print(f"memory bought for active only: {active} GB, "
      f"experts loaded: {(active - SHARED_GB) // EXPERT_GB} of {EXPERTS}")
print(f"memory needed: {total} GB + {KV_GB} GB KV cache = {total + KV_GB} GB")

# Load on real traffic: half of it is hex pieces that all like expert 3.
random.seed(7)
traffic = []
for n in range(400):
    logits = [random.gauss(0, 1) for _ in range(EXPERTS)]
    if n % 2 == 0:
        logits[2] += 4                           # a hex token: expert 3 wins
    traffic.append(logits)


def loads(bias=None):
    count = [0] * EXPERTS
    for logits in traffic:
        for k in route(logits, bias)[1]:
            count[k] += 1
    return [c / (len(traffic) * TOP) for c in count]


before = loads()
print("load without balancing:", [round(v, 3) for v in before])

# The fix: a balancing rule. Each round, nudge an expert's pick bias down
# when it gets more than its share of 1/8, and up when it gets less.
bias = [0.0] * EXPERTS
for _ in range(200):
    load = loads(bias)
    bias = [b + 0.05 * (1 / EXPERTS - l) for b, l in zip(bias, load)]
after = loads(bias)
print("load with balancing:   ", [round(v, 3) for v in after])
print(f"busiest expert: {max(before):.1%} before, {max(after):.1%} after")
