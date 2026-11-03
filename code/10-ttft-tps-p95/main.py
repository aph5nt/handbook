"""TTFT, TPS and p95: replay the lesson "TTFT, TPS and p95: split the wait".

Example numbers, not a measurement. Standard library only.
Run: python3 main.py
"""
import math

def ttft(queue_s, prefill_s):
    """Time to first token: queue wait plus prefill."""
    return queue_s + prefill_s

def end_to_end(ttft_s, output_tokens, tps):
    """TTFT plus the time decode needs for the answer."""
    return ttft_s + output_tokens / tps

def percentile(values, p):
    """Nearest rank: the smallest value with p% of values at or below it."""
    ordered = sorted(values)
    rank = math.ceil(p / 100 * len(ordered))
    return ordered[rank - 1]

print("Tab 1: one request")
t = ttft(queue_s=0.6, prefill_s=0.6)
print(f"TTFT = 0.6 queue + 0.6 prefill = {t:.1f} s")
print(f"decode = 200 tokens / 50 TPS = {200 / 50:.1f} s")
print(f"end-to-end = {end_to_end(t, 200, 50):.1f} s")

# TTFT of 20 requests at peak, in seconds.
before = [.3, .35, .4, .4, .45, .45, .5, .5, .5, .55,
          .55, .6, .6, .7, .8, .9, 1.2, 2.5, 4, 4.6]
after = [.3, .35, .4, .4, .45, .45, .5, .5, .5, .55,
         .55, .6, .6, .65, .7, .75, .8, .9, 1.2, 1.3]

print()
print("Tab 2: 20 requests at peak")
print(f"average TTFT = {sum(before) / len(before):.2f} s")
print(f"p50 = {percentile(before, 50)} s (request 10 of 20, sorted)")
print(f"p95 = {percentile(before, 95)} s (request 19 of 20, sorted)")
print(f"after trim and queue cap: p50 = {percentile(after, 50)} s, p95 = {percentile(after, 95)} s")
