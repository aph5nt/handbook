"""Rate limits and retries: the 09:00 burst against a hosted model.

Replays the lesson: instant retries grow a storm; retry-after, exponential
backoff with seeded jitter, a cap of three tries, a fallback model, and an
idempotency key fix it. Standard library only. Example numbers, not a
measurement.
"""
import random

CAPACITY = 4          # example: calls the provider accepts per second for this account
RETRY_AFTER = 2.0     # seconds, from the provider's retry-after header
MAX_TRIES = 3         # the retry cap
NEW_CALLS = [8, 8, 7, 8, 8]   # new agent calls arriving each second from 09:00
SEEDS = {1: 8, 2: 5, 3: 1373, 4: 2}   # one seeded jitter source per client, so the run repeats


def instant_retries(new_calls):
    """Every limited call retries in the next second, with no wait."""
    limited, waiting = [], 0
    for arrivals in new_calls:
        offered = arrivals + waiting
        waiting = max(0, offered - CAPACITY)
        limited.append(waiting)
    return limited


def backoff(attempt, rng):
    """Wait before retry number attempt+1: honour retry-after, double, add jitter."""
    base = max(RETRY_AFTER, RETRY_AFTER * 2 ** attempt)
    return round(base + rng.uniform(0, 1.5), 1)


def call_with_retries(answers, rng, fallback):
    """answers: what the provider says on each try. Returns the result and the waits."""
    waits = []
    for attempt in range(MAX_TRIES):
        status = answers[attempt]
        if status == 200:
            return "answered", waits
        if attempt + 1 < MAX_TRIES:
            waits.append(backoff(attempt, rng))
    return fallback(), waits          # after the cap: the tested fallback model


class NoteTool:
    """A tool with a side effect: it writes a note."""
    def __init__(self):
        self.notes, self.seen = [], set()

    def write(self, text, key=None):
        if key is not None and key in self.seen:
            return "already done"     # same idempotency key: no second write
        self.notes.append(text)
        if key is not None:
            self.seen.add(key)
        return "written"


print("== Tab 1: the 09:00 burst ==")
print(f"1-2. second 1: {NEW_CALLS[0]} calls, {CAPACITY} answered, {NEW_CALLS[0] - CAPACITY} get 429, retry after {RETRY_AFTER:.0f} s")
storm = instant_retries(NEW_CALLS)
print(f"3-4. instant retries, calls hitting the limit each second: {storm}")
tool = NoteTool()
tool.write("addr 0x9f3c risk note")          # the write happened, then the reply was lost
tool.write("addr 0x9f3c risk note")          # the instant retry, no key
print(f"5. retry without a key: {len(tool.notes)} notes written")

print("\n== Tab 2: backoff, jitter, fallback ==")
first = [backoff(0, random.Random(SEEDS[c])) for c in (1, 2, 3, 4)]
print(f"1. without jitter every client waits {RETRY_AFTER} s: all four retry at the same moment")
print(f"2. first waits for clients 1-4 (retry-after {RETRY_AFTER:.0f} s plus jitter): {first}")
rng3 = random.Random(SEEDS[3])
print(f"3. client 3 doubles each time: {[backoff(a, rng3) for a in range(3)]}")
result, waits = call_with_retries([429, 429, 529], random.Random(SEEDS[3]), fallback=lambda: "answered by the fallback model")
print(f"4-5. client 3 tries {MAX_TRIES} times (429, 429, 529), waits {waits}: {result}")
tool = NoteTool()
print(f"6. with key note-7f2: first {tool.write('addr 0x9f3c risk note', key='note-7f2')}, "
      f"retry {tool.write('addr 0x9f3c risk note', key='note-7f2')}, notes = {len(tool.notes)}")
