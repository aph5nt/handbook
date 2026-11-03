"""Backtracking: four queens, counting squares tried, against brute force.

Run: python3 main.py
"""
from itertools import product

N = 4
tried = 0       # squares tested with the safe check
dead_ends = 0   # rows where no column was safe


def safe(queens, col):
    """Can a queen go in the next row, at col, given queens in the rows above?"""
    row = len(queens)
    for r, c in enumerate(queens):
        if c == col or abs(c - col) == row - r:  # same column, or same diagonal
            return False
    return True


def solve(queens, log):
    global tried, dead_ends
    if len(queens) == N:                # every row has a queen: done
        return True
    placed_any = False
    for col in range(N):                # try columns from left to right
        tried += 1
        if not safe(queens, col):       # the pruning: check at once
            continue                    # no board below this square is ever built
        placed_any = True
        queens.append(col)              # choose
        log.append(f"place row {len(queens)} column {col + 1}  -> {[c + 1 for c in queens]}")
        if solve(queens, log):          # explore
            return True
        queens.pop()                    # undo exactly what was done
        log.append(f"undo  row {len(queens) + 1} column {col + 1}  -> {[c + 1 for c in queens]}")
    if not placed_any:
        dead_ends += 1
        log.append(f"dead end: no safe column in row {len(queens) + 1}")
    return False                        # this row ran out of choices: climb back


def brute_force():
    """Without pruning: build every full board, check only at the end."""
    boards = 0
    for board in product(range(N), repeat=N):
        boards += 1
        if all(safe(list(board[:r]), board[r]) for r in range(N)):
            return list(board), boards
    return None, boards


def main():
    queens, log = [], []
    solve(queens, log)
    print("\n".join(log))
    print("solution, columns by row:", [c + 1 for c in queens])
    print("squares tried:", tried, " dead ends:", dead_ends)
    board, boards = brute_force()
    print("brute force reached that solution after", boards, "full boards of", N ** N)
    print("8 queens by brute force: up to", f"{8 ** 8:,}", "boards")


if __name__ == "__main__":
    main()
