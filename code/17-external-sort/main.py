"""External sort with real files: twelve numbers on disk, memory for only four.

Run: python3 main.py
"""
import heapq
import math
import os
import tempfile


def make_runs(numbers, memory, folder):
    paths = []
    for start in range(0, len(numbers), memory):
        chunk = numbers[start:start + memory]    # all that fits in memory
        chunk.sort()                             # any fast in-memory sort
        path = os.path.join(folder, f"run{len(paths) + 1}.txt")
        with open(path, "w") as f:
            f.write("\n".join(map(str, chunk)) + "\n")
        paths.append(path)
    return paths


def merge_runs(paths):
    files = [open(p) for p in paths]
    heap = []
    for run, f in enumerate(files):              # one head per run, not the whole file
        heapq.heappush(heap, (int(f.readline()), run))
    output, trace = [], []
    while heap:
        trace.append(sorted(v for v, _ in heap))
        value, run = heapq.heappop(heap)         # the smallest number left anywhere
        output.append(value)
        line = files[run].readline()             # refill from the run it came from
        if line:
            heapq.heappush(heap, (int(line), run))
    for f in files:
        f.close()
    return output, trace


def passes_needed(runs, fan_in):
    passes = 0
    while runs > 1:
        runs = math.ceil(runs / fan_in)          # each pass reads the whole file once
        passes += 1
    return passes


def main():
    numbers = [9, 2, 7, 4, 8, 1, 12, 5, 3, 11, 6, 10]
    with tempfile.TemporaryDirectory() as folder:
        paths = make_runs(numbers, 4, folder)
        for p in paths:
            print(os.path.basename(p), open(p).read().split())
        output, trace = merge_runs(paths)
    print("heap before each of the first three pops:", trace[:3])
    print("output:", output)
    print("1,000 runs, merging 100 at a time:", passes_needed(1000, 100), "merge passes")
    print("1,000 runs, merging 1,000 at a time:", passes_needed(1000, 1000), "merge pass")


if __name__ == "__main__":
    main()
