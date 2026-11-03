"""Line shapes: array, linked list, queue, ring buffer and stack, counting moves.

Replays the lesson's block hashes, here shown as block numbers.
Run: python3 main.py
"""


class Array:
    def __init__(self, cap):
        self.slots, self.n, self.moves = [None] * cap, 0, 0

    def read(self, k):
        return self.slots[k]                 # start + k x size: one jump

    def append(self, x):
        if self.n == len(self.slots):        # full: new block, twice as big
            bigger = [None] * (2 * len(self.slots))
            bigger[:self.n] = self.slots
            self.moves += self.n             # every item copied across
            self.slots = bigger
        self.slots[self.n] = x
        self.n += 1

    def insert_front(self, x):
        self.append(None)                    # make sure there is room
        for k in range(self.n - 1, 0, -1):   # every item shifts one slot right
            self.slots[k] = self.slots[k - 1]
            self.moves += 1
        self.slots[0] = x

    def pop_front(self):
        x = self.slots[0]
        for k in range(1, self.n):           # every other item shifts one slot left
            self.slots[k - 1] = self.slots[k]
            self.moves += 1
        self.n -= 1
        self.slots[self.n] = None
        return x


class Node:
    def __init__(self, value, next_node=None):
        self.value, self.next = value, next_node


def list_read(head, k):
    cur, hops = head, 1
    for _ in range(k):                       # one hop per node: no jump possible
        cur = cur.next
        hops += 1
    return cur.value, hops


class RingBuffer:
    def __init__(self, cap):
        self.slots, self.front, self.count = [None] * cap, 0, 0

    def push(self, x):
        if self.count == len(self.slots):    # full: the producer must wait
            return False
        back = (self.front + self.count) % len(self.slots)  # next free slot, wraps
        self.slots[back] = x
        self.count += 1
        return True

    def pop(self):
        x = self.slots[self.front]
        self.slots[self.front] = None
        self.front = (self.front + 1) % len(self.slots)     # move the index, not the data
        self.count -= 1
        return x


def main():
    blocks = [9501, 9502, 9503, 9504, 9505, 9506]
    a = Array(8)
    for b in blocks:
        a.append(b)
    print("read slot 4:", a.read(4))
    a.append(9507)
    a.append(9508)
    print("after 9507, 9508: moves", a.moves, "room", len(a.slots))
    a.append(9509)
    print("append 9509: copied", a.moves, "into room", len(a.slots))
    before = a.moves
    a.insert_front(9500)
    print("insert 9500 at the front: moves", a.moves - before, a.slots[:a.n])

    head = None
    for b in reversed([9501, 9502, 9503, 9504, 9505]):
        head = Node(b, head)
    head = Node(9500, head)                  # new head: one pointer change
    print("list read node 5 (index 4):", list_read(head, 4))

    q = Array(8)
    for b in blocks:
        q.append(b)
    for _ in range(2):
        before = q.moves
        print("array pop front:", q.pop_front(), "moves", q.moves - before)

    r = RingBuffer(8)
    for b in blocks:
        r.push(b)
    print("ring pop:", r.pop(), "front index", r.front, "moves 0")
    for b in (9507, 9508, 9509):
        r.push(b)
    print("ring after 3 pushes:", r.slots, "front", r.front, "count", r.count)
    print("ring push 9510 returns", r.push(9510), ": full, fetch must wait")

    stack = []
    for b in blocks:
        stack.append(b)                      # push at the top
    print("stack pop:", stack.pop(), "the last one pushed")


if __name__ == "__main__":
    main()
