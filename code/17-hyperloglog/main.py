"""HyperLogLog: the lesson's toy with 8-bit hashes, then the real thing with 16,384 registers.

Everything uses sha256, so every run prints the same numbers.
Run: python3 main.py
"""
import hashlib
import math


def hash_bits(key, bits):
    """The first `bits` bits of sha256(key), as text like 10110010."""
    digest = int.from_bytes(hashlib.sha256(key.encode()).digest(), "big")
    return format(digest >> (256 - bits), f"0{bits}b")


def leading_zeros(bits):
    return len(bits) - len(bits.lstrip("0"))


def one_counter(keys):
    """A single counter: the longest run of leading zeros seen so far."""
    longest, rows = 0, []
    for key in keys:
        bits = hash_bits(key, 8)
        zeros = leading_zeros(bits)
        longest = max(longest, zeros)        # keep only the biggest
        rows.append((key, bits, zeros, longest, 2 ** longest))
    return rows


class HyperLogLog:
    def __init__(self, p, hash_size):
        self.p, self.hash_size = p, hash_size
        self.m = 2 ** p                      # number of registers
        self.reg = [0] * self.m

    def add(self, key):
        bits = hash_bits(key, self.hash_size)
        index = int(bits[:self.p], 2)        # the first p bits pick a register
        rank = leading_zeros(bits[self.p:]) + 1   # zeros in the rest, plus 1
        self.reg[index] = max(self.reg[index], rank)   # keep only the biggest
        return index, rank

    def estimate(self):
        alpha = 0.7213 / (1 + 1.079 / self.m)
        harmonic = sum(2.0 ** -r for r in self.reg)    # damps one lucky register
        raw = alpha * self.m * self.m / harmonic
        empty = self.reg.count(0)
        if raw <= 2.5 * self.m and empty:    # few keys: count the empty registers
            return self.m * math.log(self.m / empty)
        return raw

    def merge(self, other):
        for i, r in enumerate(other.reg):    # max of each pair: no key is re-read
            self.reg[i] = max(self.reg[i], r)


def main():
    print("one counter: keep the longest run of leading zeros")
    for key, bits, zeros, longest, guess in one_counter(("0x021d", "0x0000", "0x0068", "0x002e", "0x010a", "0x0068", "0x000c")):
        print(f"  {key} -> {bits}: {zeros} zeros, longest {longest}, estimate 2^{longest} = {guess}")

    print("8 registers: first 3 bits pick the register")
    day1 = HyperLogLog(3, 8)
    for key in ("0x00ed", "0x021d", "0x001d", "0x0124", "0x0030", "0x0018"):
        index, rank = day1.add(key)
        print(f"  {key} -> {hash_bits(key, 8)}: register {index}, rank {rank}, registers {day1.reg}")
    print("  estimate from the registers:", round(day1.estimate(), 1), "real: 6")

    day2 = HyperLogLog(3, 8)
    for i in range(4):
        day2.add(f"0x{0x1000 + i:04x}")
    print("  day 2 registers:", day2.reg)
    day1.merge(day2)
    print("  merged, max of each pair:", day1.reg)

    print("the real size: 16,384 registers, 64-bit hashes")
    a, b, union = HyperLogLog(14, 64), HyperLogLog(14, 64), 300_000
    for i in range(200_000):
        a.add(f"wallet-{i}")
    for i in range(100_000, 300_000):        # overlaps the first day by 100,000
        b.add(f"wallet-{i}")
    print(f"  day 1: real 200,000, estimate {a.estimate():,.0f}, error {abs(a.estimate() - 200_000) / 200_000:.2%}")
    a.merge(b)
    print(f"  merged: real {union:,}, estimate {a.estimate():,.0f}, error {abs(a.estimate() - union) / union:.2%}")
    print(f"  expected error 1.04 / sqrt(16384) = {1.04 / math.sqrt(16384):.2%}, memory 16,384 x 6 bits = {16384 * 6 // 8 // 1024} KB")


if __name__ == "__main__":
    main()
