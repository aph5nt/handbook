"""LoRA and adapters: replay the lesson "LoRA: two thin matrices beside a frozen model".

Part 1 counts the numbers in one big weight matrix W and in its adapter (A, B).
Part 2 trains a tiny LoRA in plain lists: W never changes, only A and B do.
Part 3 shows the base-version check that refuses an adapter built for another base.
Toy sizes in part 2 (d = 4, rank 1). Standard library only. Run: python3 main.py
"""

D, RANK = 4096, 16          # one weight matrix of a small model, and the LoRA rank
ADAPTED = 128               # adapted matrices: 4 attention matrices x 32 layers
BYTES = 2                   # 16-bit numbers


def params(d, r):
    """Numbers in W, and in A (r x d) plus B (d x r)."""
    return d * d, 2 * d * r


def matvec(m, x):
    return [sum(a * b for a, b in zip(row, x)) for row in m]


def forward(w, a, b, x):
    """y = W x + B (A x): the base output plus the adapter's nudge."""
    base = matvec(w, x)
    delta = matvec(b, matvec(a, x))
    return [p + q for p, q in zip(base, delta)]


def train_step(w, a, b, x, target, lr):
    """One gradient step on A and B only. W is read, never written."""
    y = forward(w, a, b, x)
    err = [yi - ti for yi, ti in zip(y, target)]
    ax = matvec(a, x)
    grad_ax = [sum(err[i] * b[i][j] for i in range(len(b))) for j in range(len(a))]
    for i in range(len(b)):
        for j in range(len(b[0])):
            b[i][j] -= lr * err[i] * ax[j]
    for j in range(len(a)):
        for k in range(len(a[0])):
            a[j][k] -= lr * grad_ax[j] * x[k]
    return sum(e * e for e in err)


def load_adapter(adapter, base):
    """Refuse an adapter trained on another base model."""
    if adapter["base_version"] != base["version"]:
        raise ValueError(f"adapter built for {adapter['base_version']}, base is {base['version']}")
    return adapter


w_params, ab_params = params(D, RANK)
print("Part 1: sizes")
print(f"  W: {w_params:,} numbers, frozen")
print(f"  A + B at rank {RANK}: {ab_params:,} numbers = {ab_params / w_params:.2%} of W")
print(f"  whole adapter, {ADAPTED} matrices: {ADAPTED * ab_params * BYTES / 1e6:.1f} MB")
print(f"  base weights of those matrices: {ADAPTED * w_params * BYTES / 1e9:.1f} GB")

print("Part 2: train only A and B (toy, d = 4, rank 1)")
W = [[1.0 if i == j else 0.0 for j in range(4)] for i in range(4)]   # frozen base
W_before = [row[:] for row in W]
A = [[0.1, 0.1, 0.1, 0.1]]            # rank x d
B = [[0.0], [0.0], [0.0], [0.0]]      # d x rank, starts at zero: no nudge yet
examples = [([1, 0, 0, 0], [1, 0.5, 0, 0]), ([0, 1, 0, 0], [0, 1.5, 0, 0])]  # wanted shape
for epoch in range(1, 301):
    loss = sum(train_step(W, A, B, x, t, 0.1) for x, t in examples)
    if epoch in (1, 300):
        print(f"  epoch {epoch}: loss {loss:.4f}")
print("  W unchanged:", W == W_before)
print("  output for [1,0,0,0]:", [round(v, 2) for v in forward(W, A, B, [1, 0, 0, 0])])

print("Part 3: the base-version check")
adapter = {"name": "report-format", "base_version": "v1", "A": A, "B": B}
for base in ({"version": "v1"}, {"version": "v2"}):
    try:
        load_adapter(adapter, base)
        print(f"  base {base['version']}: loaded")
    except ValueError as e:
        print(f"  base {base['version']}: refused, {e}")
