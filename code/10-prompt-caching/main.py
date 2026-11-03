"""Prompt caching: replay the lesson "Prompt caching: pay for the stable prefix once".

A prefix cache keyed by a hash of the exact prefix bytes. Billing in example
units: 1 unit per 1,000 full input tokens, 0.1 per 1,000 cached tokens.
Real APIs also charge a little extra for the call that writes the cache; the
lesson leaves that out. Standard library only, no network. Run: python3 main.py
"""
import hashlib

PREFIX_TOKENS = 20_000          # system prompt, tool list and loaded skills
FULL, CACHED = 1.0, 0.1         # units per 1,000 input tokens


class PrefixCache:
    def __init__(self):
        self.store = set()
        self.billed = 0.0
        self.hits = 0
        self.calls = 0

    def call(self, prefix, suffix):
        self.calls += 1
        key = hashlib.sha256(prefix.encode()).hexdigest()
        if key in self.store:
            self.billed += PREFIX_TOKENS / 1000 * CACHED   # 2 units
            self.hits += 1
            return "hit", key       # only the suffix is prefilled
        self.billed += PREFIX_TOKENS / 1000 * FULL         # 20 units
        self.store.add(key)         # stored for the next call
        return "miss", key          # the whole prompt is prefilled


STABLE = "You are an on-chain investigator. Write every address in full. [tools] [skills]"
QUESTIONS = ["Who funded this wallet?", "Where did it send the funds?",
             "Which exchange did it reach?"]


def prompt_with_time_on_top(now, question):
    return f"Now: {now}\n" + STABLE, question            # the failure


def prompt_with_time_at_end(now, question):
    return STABLE, f"{question}\nNow: {now}"             # the fix


def session(title, build, times):
    cache = PrefixCache()
    print(title)
    for now, q in zip(times, QUESTIONS):
        prefix, suffix = build(now, q)
        result, key = cache.call(prefix, suffix)
        print(f"  call {cache.calls} at {now}: key {key[:8]}  {result:4}  "
              f"input billed so far: {cache.billed:.0f} units")
    print(f"  hit rate: {cache.hits} of {cache.calls} calls")


TIMES = ["09:14:03", "09:14:04", "09:14:05"]
session("Tab 1: same prefix, cache hit", prompt_with_time_at_end, TIMES)
print()
session("Tab 2: the time at the top, one changed byte per call", prompt_with_time_on_top, TIMES)
print()
session("Tab 2 fix: the time moved to the end", prompt_with_time_at_end, TIMES)
