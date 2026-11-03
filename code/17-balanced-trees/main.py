"""Balanced trees: AVL and red-black, inserting the lesson's sorted keys 10 to 50.

Run: python3 main.py
"""


class AVL:
    def __init__(self, key):
        self.key, self.left, self.right, self.h = key, None, None, 1


def h(n):
    return n.h if n else 0


def balance(n):
    return h(n.left) - h(n.right)            # the rule: -1, 0 or 1


def fix_height(n):
    n.h = 1 + max(h(n.left), h(n.right))


def rotate_left(x):
    y = x.right                              # y is lifted into x's place
    x.right, y.left = y.left, x              # x becomes y's left child
    fix_height(x)
    fix_height(y)
    return y


def rotate_right(x):
    y = x.left
    x.left, y.right = y.right, x
    fix_height(x)
    fix_height(y)
    return y


def avl_insert(n, key, log):
    if n is None:
        return AVL(key)
    if key < n.key:
        n.left = avl_insert(n.left, key, log)
    else:
        n.right = avl_insert(n.right, key, log)
    fix_height(n)                            # on the way back up: recheck each node
    b = balance(n)
    if b < -1:                               # right side too deep
        log.append(f"balance of {n.key} is {b}: rotate left at {n.key}")
        if balance(n.right) > 0:             # inner grandchild: two rotations
            n.right = rotate_right(n.right)
        return rotate_left(n)
    if b > 1:                                # left side too deep: the mirror
        log.append(f"balance of {n.key} is {b}: rotate right at {n.key}")
        if balance(n.left) < 0:
            n.left = rotate_left(n.left)
        return rotate_right(n)
    return n


def avl_shape(n):
    return None if n is None else (n.key, avl_shape(n.left), avl_shape(n.right))


RED, BLACK = "red", "black"
L, R = 0, 1


class RB:
    def __init__(self, key, parent=None):
        self.key, self.color, self.parent, self.kid = key, RED, parent, [None, None]  # new nodes are red


class RedBlackTree:
    def __init__(self):
        self.root, self.log = None, []

    def rotate(self, x, d):                  # d = L lifts x's right child
        y = x.kid[1 - d]
        x.kid[1 - d] = y.kid[d]
        if y.kid[d]:
            y.kid[d].parent = x
        y.parent = x.parent
        if x.parent is None:
            self.root = y
        else:
            x.parent.kid[x.parent.kid.index(x)] = y
        y.kid[d], x.parent = x, y

    def insert(self, key):
        parent, n = None, self.root
        while n:                             # a plain search tree insert first
            parent, n = n, n.kid[R if key > n.key else L]
        node = RB(key, parent)
        if parent is None:
            self.root = node
        else:
            parent.kid[R if key > parent.key else L] = node
        self.fix(node)

    def fix(self, n):
        while n.parent and n.parent.color == RED:    # red under red breaks the rule
            p, g = n.parent, n.parent.parent
            d = g.kid.index(p)
            uncle = g.kid[1 - d]
            if uncle and uncle.color == RED:         # red uncle: recolour only
                p.color = uncle.color = BLACK
                g.color = RED
                self.log.append(f"{n.key}: red uncle {uncle.key}, recolour")
                n = g
            else:                                    # empty or black uncle: rotate
                if p.kid.index(n) != d:              # inner child: straighten first
                    self.rotate(p, d)
                    n, p = p, n
                self.rotate(g, 1 - d)
                p.color, g.color = BLACK, RED
                self.log.append(f"{n.key}: no red uncle, rotate at {g.key}, recolour")
        self.root.color = BLACK                      # the root is always black


def rb_shape(n):
    return None if n is None else (f"{n.key} {n.color}", rb_shape(n.kid[L]), rb_shape(n.kid[R]))


def main():
    root, log = None, []
    for key in (10, 20, 30, 40, 50):
        log.clear()
        root = avl_insert(root, key, log)
        print(f"AVL insert {key}: balance of root {balance(root)}, {log or 'no rotation'}, shape {avl_shape(root)}")
    print("AVL height after 5 sorted keys:", root.h, "(a plain tree would be 5)")

    t = RedBlackTree()
    for key in (10, 20, 30, 40, 50):
        t.log.clear()
        t.insert(key)
        print(f"red-black insert {key}: {t.log or 'no fix needed'}, shape {rb_shape(t.root)}")

    big, count = None, 100_000
    for key in range(count):
        big = avl_insert(big, key, [])
    print(f"AVL over {count:,} sorted keys: height {big.h}")


if __name__ == "__main__":
    main()
