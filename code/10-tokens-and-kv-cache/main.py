"""Tokens, prefill and the KV cache: replay the lesson of the same name.

A toy tokenizer (greedy longest match in a small vocabulary), a KV cache that
grows by one entry per token, and a prefix cache keyed by the prefix bytes.
Standard library only, no network. Run: python3 main.py
"""
import hashlib

VOCAB = {"Who", " sent", " money", " to", " on", " robinhood", "?", " 0x", "..."}
VOCAB |= {f"{i:02x}" for i in range(256)}      # every pair of hex digits


def tokenize(text):
    out, i = [], 0
    while i < len(text):
        for size in range(len(text) - i, 0, -1):   # longest piece first
            piece = text[i:i + size]
            if piece in VOCAB or size == 1:        # one character is the fallback
                out.append(piece)
                i += size
                break
    return out


ANSWER = ["The", " sender", " was", " an", " exchange", " hot", " wallet", "."]


def serve(prompt_tokens):
    kv = [("kv", t) for t in prompt_tokens]       # prefill: one entry per prompt token
    print(f"  prefill: {len(prompt_tokens)} tokens in one pass, KV cache = {len(kv)}")
    for n, tok in enumerate(ANSWER, 1):           # decode: one token per step
        kv.append(("kv", tok))                    # the cache grows by one
        if n in (1, 4, len(ANSWER)):
            print(f"  decode step {n}: wrote {tok!r}, KV cache = {len(kv)}")
    return kv


def kv_entries(context_tokens, users):
    return context_tokens * users                 # one cache per user, one entry per token


cache = {}


def call(prefix, question):
    key = hashlib.sha256(prefix.encode()).hexdigest()
    if key in cache:
        return f"hit  (key {key[:8]}): prefill the question only"
    cache[key] = "processed prefix"               # its KV cache, kept a few minutes
    return f"miss (key {key[:8]}): full prefill"


question = "Who sent money to 0x9f3c...0071 on robinhood?"
print("Tab 1: prefill and decode")
print(f"  text: {len(question)} characters")
toks = tokenize(question)
print(f"  tokens ({len(toks)}): {toks}")
print(f"  the address alone: {len(toks[4:10])} tokens: {toks[4:10]}")
serve(toks)
print(f"  failure: 100,000-token context x 50 users = {kv_entries(100_000, 50):,} cache entries")

print()
print("Tab 2: prompt caching")
STABLE = "[system prompt][tools][skills]"
print("  call 1:", call(STABLE, "Who funded this wallet?"))
print("  call 2:", call(STABLE, "Where did the funds go?"))
print("  time on top:", call("Time 09:41\n" + STABLE, "Which exchange?"))
print("  time moved last:", call(STABLE, "Which exchange? Time 09:41"))
