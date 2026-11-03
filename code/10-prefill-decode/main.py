"""Prefill and decode: replay the lesson "Prefill reads the prompt at once,
decode writes one token at a time".

Tab 1 counts KV cache entries for one request: 12 prompt tokens, 8 answer tokens.
Tab 2 shares a GPU's KV cache budget between requests. Sizes are example
numbers in GB, not a measurement. Standard library only. Run: python3 main.py
"""

KV_BUDGET = 18      # GB of GPU memory left for KV caches after the weights
MAX_CONTEXT = 4     # GB per request; the fix caps a request's cache at this


def prefill(prompt):
    """One parallel pass: every prompt token gets its keys and values at once."""
    return [("kv", tok) for tok in prompt]


def run_one_request(prompt, answer_len):
    kv = prefill(prompt)
    print(f"prefill: {len(prompt)} prompt tokens in one pass, KV cache = {len(kv)} entries")
    answer = []
    for step in range(1, answer_len + 1):
        tok = f"a{step}"                  # the step reads all of kv to pick this token
        answer.append(tok)
        kv.append(("kv", tok))            # the cache grows by one entry per new token
        if step == 1:
            print(f"first token {tok}: time to first token ends, KV cache = {len(kv)}")
        elif step in (4, answer_len):
            print(f"decode step {step}: read {len(kv) - 1} entries, KV cache = {len(kv)}")
    kv.clear()                            # done: memory goes back to the pool
    print(f"done: cache freed, KV cache = {len(kv)}")


class Card:
    def __init__(self, cap_on):
        self.cap_on = cap_on
        self.batch = {}                   # request name -> GB of KV cache
        self.paused = []

    def used(self):
        return sum(self.batch.values())

    def admit(self, name, need):
        if self.cap_on and need > MAX_CONTEXT:
            print(f"  {name} needs {need} GB, capped to {MAX_CONTEXT} GB")
            need = MAX_CONTEXT
        while self.used() + need > KV_BUDGET:
            print(f"  memory full: {self.used()} + {need} > {KV_BUDGET} GB")
            newest = [n for n in self.batch][-1]
            self.paused.append(newest)    # its cache is freed, it waits in the queue
            print(f"  pause {newest}, frees {self.batch.pop(newest)} GB")
        self.batch[name] = need

    def show(self, title):
        print(f"{title}: running {list(self.batch)} using {self.used()} of {KV_BUDGET} GB,"
              f" paused {self.paused}")


print("Tab 1: prefill, then decode")
run_one_request([f"p{k}" for k in range(1, 13)], answer_len=8)

print()
print("Tab 2: one card, shared (cap switched off)")
card = Card(cap_on=False)
for k in range(1, 5):
    card.admit(f"request {k}", 3)
card.show("step 1, four short requests")
print("step 2, request 5 arrives with a long prompt: needs 10 GB")
card.admit("request 5", 10)
card.show("step 4, the batch shrank")

print()
print("Fix: cap the context at 4 GB per request")
card = Card(cap_on=True)
for k in range(1, 5):
    card.admit(f"request {k}", 3)
card.admit("request 5", 10)
card.show("step 5, all five run")
