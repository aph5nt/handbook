"""Batch inference: a nightly job labels 100,000 addresses.

Replays the lesson: live calls against a rate limit, then one batch file
whose results come back shuffled, matched by request ID, failures retried.
Standard library only. Prices and limits are example numbers.
"""
import random

N = 100_000
LIMIT_PER_MIN = 1_000          # example live limit: requests per minute
LIVE_PRICE, BATCH_PRICE = 1.0, 0.5   # example units per request
KINDS = ["exchange", "mixer", "wallet", "contract"]


def true_label(address_id):
    """What the model answers for this address (deterministic stand-in)."""
    return KINDS[int(address_id.split("-")[1]) * 7 % 4]


def live_minute(burst):
    """One minute of live calls: the limit lets some through, the rest get 429."""
    ok = min(burst, LIMIT_PER_MIN)
    return ok, burst - ok


def write_batch(ids):
    """One line per request. The custom_id is how a result finds its request."""
    return [{"custom_id": i, "params": {"prompt": f"Classify {i}"}} for i in ids]


def provider_run(requests, rng, failing):
    """The provider runs the file later and returns lines in any order."""
    out = []
    for r in requests:
        cid = r["custom_id"]
        if cid in failing:
            out.append({"custom_id": cid, "result": {"type": "errored"}})
        else:
            out.append({"custom_id": cid, "result": {"type": "succeeded", "label": true_label(cid)}})
    rng.shuffle(out)
    return out


def match_by_position(requests, results):
    return {req["custom_id"]: res["result"].get("label") for req, res in zip(requests, results)}


def match_by_id(results):
    labels, failed = {}, []
    for res in results:
        if res["result"]["type"] == "succeeded":
            labels[res["custom_id"]] = res["result"]["label"]
        else:
            failed.append(res["custom_id"])
    return labels, failed


def route(person_waits):
    return "live" if person_waits else "batch"


ids = [f"addr-{k:05d}" for k in range(1, N + 1)]
rng = random.Random(3)

print("== Tab 1: the nightly job ==")
print(f"1. live: {N:,} calls x {LIVE_PRICE} = {N * LIVE_PRICE:,.0f} units")
ok, limited = live_minute(5_000)
print(f"2. first minute, burst of 5,000: {ok:,} answered, {limited:,} get 429 too many requests")
print(f"   at {LIMIT_PER_MIN:,} per minute the job needs at least {N // LIMIT_PER_MIN} minutes plus retries")
requests = write_batch(ids)
print(f"3. batch file: {len(requests):,} lines, first {requests[0]['custom_id']}")
print(f"4. submitted once: {len(requests):,} x {BATCH_PRICE} = {len(requests) * BATCH_PRICE:,.0f} units")
results = provider_run(requests, rng, failing={"addr-00002", "addr-00005"})
print(f"5-6. result file: {len(results):,} lines, first line is {results[0]['custom_id']}")
labels, failed = match_by_id(results)
print(f"7. matched by ID: {len(labels):,} labels, failed {failed}")
retry = provider_run(write_batch(failed), rng, failing=set())
more, still = match_by_id(retry)
labels.update(more)
print(f"   retry batch of {len(failed)}: {len(more)} ok, {len(still)} still failing, total {len(labels):,}")

print("\n== Tab 2: when batch goes wrong ==")
results = provider_run(requests, random.Random(3), failing=set())
print(f"1. shuffled: result line 1 is {results[0]['custom_id']}, request line 1 is {requests[0]['custom_id']}")
by_pos = match_by_position(requests, results)
wrong = sum(by_pos[i] != true_label(i) for i in ids)
print(f"2. matched by position: {wrong:,} of {N:,} labels wrong")
by_id, _ = match_by_id(results)
wrong = sum(by_id[i] != true_label(i) for i in ids)
print(f"3. matched by ID: {wrong} of {N:,} labels wrong")
print("4. investigator question sent to batch: waits up to the 24-hour batch window")
print(f"5. route(person_waits=True) = {route(True)}, route(person_waits=False) = {route(False)}")
