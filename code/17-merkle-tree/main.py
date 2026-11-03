"""A Merkle tree over the lesson's four transactions: root, proof, verify, tamper.

Run: python3 main.py
"""
import hashlib


def h(data):
    return hashlib.sha256(data).digest()


def build_levels(txs):
    level = [h(tx) for tx in txs]            # the leaves: H1, H2, H3, H4
    levels = [level]
    while len(level) > 1:
        if len(level) % 2:                   # an odd count: repeat the last hash
            level = level + [level[-1]]
        level = [h(level[i] + level[i + 1]) for i in range(0, len(level), 2)]
        levels.append(level)                 # each level halves the hashes
    return levels


def make_proof(levels, index):
    proof = []
    for level in levels[:-1]:                # one sibling per level
        sibling = index ^ 1                  # the other half of this pair
        if sibling == len(level):
            sibling = index
        proof.append((level[sibling], "left" if sibling < index else "right"))
        index //= 2                          # move up to the parent
    return proof


def verify(tx, proof, root):
    current = h(tx)                          # the client hashes tx itself
    for sibling, side in proof:              # then climbs with the sent hashes
        pair = sibling + current if side == "left" else current + sibling
        current = h(pair)
    return current == root, current          # compare with the header's root


def short(digest):
    return digest.hex()[:8]


def main():
    txs = [b"tx1", b"tx2", b"tx3", b"tx4"]
    levels = build_levels(txs)
    print("H1 to H4:", [short(x) for x in levels[0]])
    print("H12, H34:", [short(x) for x in levels[1]])
    root = levels[2][0]
    print("root:", short(root))
    proof = make_proof(levels, 2)
    print("proof for tx3:", [(short(x), side) for x, side in proof])
    ok, got = verify(b"tx3", proof, root)
    print("verify tx3:", ok, "computed", short(got))
    bad, got = verify(b"tx9", proof, root)
    forged = h(b"tx9")
    print("one byte changed, tx3 becomes tx9: H3", short(forged), "H34", short(h(forged + levels[0][3])))
    print("verify the changed tx:", bad, "computed", short(got), "header says", short(root))
    big = build_levels([str(i).encode() for i in range(1_000_000)])
    print("a million transactions: tree levels", len(big), "proof hashes", len(make_proof(big, 123_456)))


if __name__ == "__main__":
    main()
