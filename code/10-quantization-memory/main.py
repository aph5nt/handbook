"""Quantization and memory: replay the lesson "fewer bits, smaller model, some quality cost".

Tab 1 sizes a 70-billion-weight model at 16, 8 and 4 bits against two 24 GB
cards, then adds the KV cache. The model shape (80 layers, 8 key-value heads
of width 128) is a common 70B layout. Tab 2 quantizes 1,000 toy weights and
measures the round-trip error. Standard library only. Run: python3 main.py
"""

PARAMS = 70e9
CARD_GB = 2 * 24
LAYERS, KV_HEADS, HEAD_DIM, KV_BYTES = 80, 8, 128, 2


def weights_gb(bits):
    return PARAMS * bits / 8 / 1e9           # parameters times bytes per weight


def kv_per_token():
    return 2 * LAYERS * KV_HEADS * HEAD_DIM * KV_BYTES   # keys and values, bytes


def kv_gb(users, tokens_each):
    return kv_per_token() * users * tokens_each / 1e9


def fits(bits, users=0, tokens_each=0):
    total = weights_gb(bits) + kv_gb(users, tokens_each)
    return total, total <= CARD_GB


def max_live_tokens(bits):
    """The cap: how many tokens of KV cache fit beside the weights."""
    return int((CARD_GB - weights_gb(bits)) * 1e9 / kv_per_token())


def quantize(ws, bits):
    """Symmetric absmax quantization of one group: scale, round, scale back."""
    scale = max(abs(w) for w in ws) / (2 ** (bits - 1) - 1)
    return [round(w / scale) * scale for w in ws]


print("Tab 1: make it fit")
for bits in (16, 8, 4):
    total, ok = fits(bits)
    print(f"  {bits:2}-bit: weights {weights_gb(bits):5.0f} GB  {'fits' if ok else 'does not fit'} in {CARD_GB} GB")
print(f"KV cache per token: {kv_per_token() / 1e6:.2f} MB")
for label, users, each in [("4 users x 6,000 tokens", 4, 6000),
                           ("peak: 20 users x 4,600 tokens", 20, 4600),
                           ("capped: 8 users x 4,500 tokens", 8, 4500)]:
    total, ok = fits(4, users, each)
    print(f"  {label}: KV {kv_gb(users, each):4.1f} GB, total {total:4.1f} GB -> {'fits' if ok else 'queue or reject'}")
print(f"cap at 4-bit: at most {max_live_tokens(4):,} live tokens")

print()
print("Tab 2: what it costs")
seed, ws = 7, []
for _ in range(1000):                       # deterministic toy weights in [-1, 1)
    seed = (seed * 1103515245 + 12345) % 2 ** 31
    ws.append(seed / 2 ** 30 - 1)
for bits in (8, 4, 3):
    back = quantize(ws, bits)
    err = sum(abs(a - b) for a, b in zip(ws, back)) / len(ws)
    print(f"  {bits}-bit: mean round-trip error {err:.4f}")
print(f"  3-bit weights {weights_gb(3):.0f} GB: saves {weights_gb(4) - weights_gb(3):.0f} GB more than 4-bit")
print("golden set (example numbers from the lesson, not computed here):")
print("  16-bit 96%, 8-bit 95%, 4-bit 90%, 4-bit exact numbers 78%, 3-bit 61%")
