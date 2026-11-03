"""An LSM tree: write-ahead log, memtable, flushed files, reads, compaction, write stall.

Run: python3 main.py
"""
import bisect
import hashlib


class SSTable:
    """A sorted file that never changes, with a Bloom filter in front."""

    def __init__(self, items):
        self.items = sorted(items.items())
        self.keys = [k for k, _ in self.items]
        self.filter = {b for k in self.keys for b in bloom_bits(k)}

    def get(self, key):
        i = bisect.bisect_left(self.keys, key)   # binary search inside the file
        if i < len(self.keys) and self.keys[i] == key:
            return self.items[i][1]
        return None


def bloom_bits(key, m=64, k=3):
    return [int(hashlib.sha256(f"{i}:{key}".encode()).hexdigest(), 16) % m for i in range(k)]


class LSM:
    def __init__(self, limit=4, stall_at=7):
        self.limit, self.stall_at = limit, stall_at
        self.wal, self.mem, self.files = [], {}, []  # files: newest last

    def put(self, key, value):
        if len(self.mem) == self.limit:          # no room, and the flush was refused
            return "stalled"
        self.wal.append((key, value))            # append first: survives a crash
        self.mem[key] = value                    # then the sorted table in memory
        if len(self.mem) == self.limit and len(self.files) < self.stall_at:
            self.files.append(SSTable(self.mem)) # full: one big sequential write
            self.mem, self.wal = {}, []          # the log's job is done
        return "ok"

    def get(self, key):
        checked = []
        if key in self.mem:                      # newest data first
            return self.mem[key], ["memtable"]
        for n in range(len(self.files) - 1, -1, -1):  # newest file first
            f = self.files[n]
            if not all(b in f.filter for b in bloom_bits(key)):
                checked.append(f"file {n + 1} filter says absent")
                continue                         # no disk read for this file
            checked.append(f"file {n + 1} read")
            value = f.get(key)
            if value is not None:
                return value, checked
        return None, checked

    def compact(self):
        merged = {}
        for f in self.files:                     # oldest first, so newer values win
            merged.update(f.items)
        self.files = [SSTable(merged)]           # the old k5 is dropped here


def main():
    db = LSM()
    for key in ("k5", "k2", "k9"):
        db.put(key, "v1")
    print("memtable:", sorted(db.mem), "log entries:", len(db.wal), "files:", len(db.files))
    db.put("k7", "v1")
    print("after k7: flushed file 1:", db.files[0].keys, "memtable:", sorted(db.mem), "log entries:", len(db.wal))
    for key, value in (("k5", "v2"), ("k1", "v1"), ("k8", "v1"), ("k3", "v1")):
        db.put(key, value)
    print("file 2:", db.files[1].keys, " k5 in file 1:", db.files[0].get("k5"), " k5 in file 2:", db.files[1].get("k5"))
    print("read k7:", db.get("k7"))
    print("read k5:", db.get("k5"), "newest file wins")
    db.compact()
    print("after compaction:", db.files[0].items)
    print("read k5 after compaction:", db.get("k5"))
    db = LSM()
    results = [db.put(f"x{i}", "v") for i in range(33)]
    print("writes faster than compaction: files", len(db.files), "memtable", sorted(db.mem), "log entries", len(db.wal))
    print("write number 33 says:", results[-1], " writes accepted:", results.count("ok"))


if __name__ == "__main__":
    main()
