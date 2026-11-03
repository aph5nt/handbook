"""Tensor parallel and pipeline parallel: replay the lesson "Tensor parallel and pipeline parallel".

Part 1: memory. 70 billion weights at 16 bits do not fit one 80 GB card.
Part 2: a tensor split in plain lists. Two "cards" each hold half the rows of
a layer matrix; joined, their halves give the same answer as one card.
Part 3: time per token with example costs (not measurements): tensor parallel
on a fast link, on a slow link, and pipeline parallel for one request.
Standard library only. Run: python3 main.py
"""

WEIGHTS, CARD_GB, LAYERS = 70e9, 80, 80


def gb(weights, bits):
    return weights * bits / 8 / 1e9


def matvec(m, x):
    return [sum(a * b for a, b in zip(row, x)) for row in m]


def tensor_parallel(m, x, cards):
    """Each card takes a block of rows, computes its part, then the parts are joined."""
    size = len(m) // cards
    parts = [matvec(m[c * size:(c + 1) * size], x) for c in range(cards)]
    return [v for part in parts for v in part]          # the exchange after each layer


# Example costs per layer, in milliseconds (not measurements)
LAYER_MS = 0.2          # one card computes a whole layer
NVLINK_MS = 0.01        # one exchange inside a server
NETWORK_MS = 0.5        # one exchange between servers
HANDOFF_MS = 0.5        # pipeline: one hand-off between servers


def per_token_tensor(cards, link_ms):
    return LAYERS * (LAYER_MS / cards + link_ms)        # every layer: compute part, then exchange


def per_token_pipeline(stages):
    busy = LAYERS * LAYER_MS                            # the stages run in turn for one request
    return busy + (stages - 1) * HANDOFF_MS, 1 / stages  # time, and each card's busy share


print("Part 1: memory")
print(f"  16-bit: {gb(WEIGHTS, 16):.0f} GB, one card holds {CARD_GB} GB: fits = {gb(WEIGHTS, 16) <= CARD_GB}")
print(f"  split over 2 cards: {gb(WEIGHTS, 16) / 2:.0f} GB each")
print(f"  4-bit on one card: {gb(WEIGHTS, 4):.0f} GB, fits = {gb(WEIGHTS, 4) <= CARD_GB}")

print("Part 2: a tensor split gives the same answer")
layer = [[1, 2], [3, 4], [5, 6], [7, 8]]
x = [1, 1]
print("  one card:", matvec(layer, x), " two cards:", tensor_parallel(layer, x, 2))

print("Part 3: time per token (example costs)")
print(f"  one card, if it fitted: {LAYERS * LAYER_MS:.1f} ms")
print(f"  tensor parallel, NVLink: {per_token_tensor(2, NVLINK_MS):.1f} ms, {LAYERS} exchanges")
print(f"  tensor parallel, network: {per_token_tensor(2, NETWORK_MS):.1f} ms, slower than one card")
t, share = per_token_pipeline(2)
print(f"  pipeline, one request: {t:.1f} ms, 1 hand-off, each card busy {share:.0%} of the time")
