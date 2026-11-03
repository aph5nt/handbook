"""Edit distance: the lesson's table for aeor -> aero, then what plain recursion costs.

Run: python3 main.py
"""


def edit_table(a, b):
    """D[i][j] = edits to turn the first i letters of a into the first j letters of b."""
    D = [[0] * (len(b) + 1) for _ in range(len(a) + 1)]
    for i in range(len(a) + 1):
        D[i][0] = i                       # i deletes reach the empty string
    for j in range(len(b) + 1):
        D[0][j] = j                       # j inserts from the empty string
    for i in range(1, len(a) + 1):        # row by row
        for j in range(1, len(b) + 1):    # left to right
            same = a[i - 1] == b[j - 1]
            D[i][j] = min(D[i - 1][j] + 1,                  # above + 1: delete
                          D[i][j - 1] + 1,                  # left + 1: insert
                          D[i - 1][j - 1] + (0 if same else 1))  # diagonal: match or substitute
    return D


def walk_back(D, a, b):
    """Follow the choices from the corner back to 0, 0."""
    i, j, edits = len(a), len(b), []
    while i > 0 or j > 0:
        same = i > 0 and j > 0 and a[i - 1] == b[j - 1]
        if i > 0 and j > 0 and D[i][j] == D[i - 1][j - 1] + (0 if same else 1):
            edits.append(f"keep {a[i - 1]}" if same else f"substitute {a[i - 1]} -> {b[j - 1]}")
            i, j = i - 1, j - 1
        elif i > 0 and D[i][j] == D[i - 1][j] + 1:
            edits.append(f"delete {a[i - 1]}")
            i -= 1
        else:
            edits.append(f"insert {b[j - 1]}")
            j -= 1
    return edits[::-1]


calls = 0


def plain(a, b, i, j):
    """Plain recursion: asks three smaller pairs, again and again."""
    global calls
    calls += 1
    if i == 0 or j == 0:
        return i + j
    same = a[i - 1] == b[j - 1]
    return min(plain(a, b, i - 1, j) + 1, plain(a, b, i, j - 1) + 1,
               plain(a, b, i - 1, j - 1) + (0 if same else 1))


def two_rows(a, b):
    """Only the number: keep the row above and the row being filled."""
    above = list(range(len(b) + 1))
    for i in range(1, len(a) + 1):
        row = [i]
        for j in range(1, len(b) + 1):
            same = a[i - 1] == b[j - 1]
            row.append(min(above[j] + 1, row[j - 1] + 1, above[j - 1] + (0 if same else 1)))
        above = row
    return above[-1]


def plain_calls(m, n):
    """How many calls plain recursion makes on m and n letters, without running it."""
    C = [[1] * (n + 1) for _ in range(m + 1)]
    for i in range(1, m + 1):
        for j in range(1, n + 1):
            C[i][j] = 1 + C[i - 1][j] + C[i][j - 1] + C[i - 1][j - 1]
    return C[m][n]


def main():
    a, b = "aeor", "aero"
    D = edit_table(a, b)
    print("      " + "  ".join(" " + b))
    for i, row in enumerate(D):
        print((" " + a)[i], " ", "  ".join(str(v) for v in row))
    print("corner:", D[-1][-1], " edits:", walk_back(D, a, b), " two rows:", two_rows(a, b))
    print("cells filled:", len(D) * len(D[0]))
    print("plain recursion on aeor, aero:", plain(a, b, 4, 4), "after", calls, "calls, formula", plain_calls(4, 4))
    print(f"plain recursion on two 20-letter names: {plain_calls(20, 20):,} calls, the table fills {21 * 21} cells")


if __name__ == "__main__":
    main()
