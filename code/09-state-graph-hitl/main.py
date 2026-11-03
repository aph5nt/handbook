"""State graph and a human in the loop.

Replays visual/09-state-graph-hitl.html. A tiny graph runner in plain Python:
nodes change a typed state, conditional edges read it. The server is fake.
Run: python3 main.py
"""
from dataclasses import dataclass, field


SERVER_UP = True  # tab 2 turns this off, so every statement is refused


def fake_server(statement):
    if SERVER_UP and "last 30 days" in statement:
        return [f"row {i}" for i in range(42)], None
    return [], "query_timeout"


class ChatState:
    """Only chat text. No attempts field, so the edge reads it as 0."""
    rows = []
    attempts = 0


# panel:start
@dataclass
class State:                                        # @1 typed fields, not chat text
    question: str
    statement: str = ""
    rows: list = field(default_factory=list)
    reason: str = ""
    attempts: int = 0                               # @1 a number an edge can read
    draft: str = ""
    approved: bool = False


def generate(s):
    s.statement = f"walk every sender of {s.question}"  # @2


def run_query(s):
    s.rows, s.reason = fake_server(s.statement)     # @3


def rewrite(s):
    days = [30, 14, 7][min(s.attempts, 2)]          # narrower on each try
    s.statement = f"walk the senders of {s.question}, last {days} days"  # @5
    s.attempts += 1                                 # @5


def answer(s):
    s.draft = f"{len(s.rows)} senders, full addresses"  # @6


def after_run(s):                                   # @4 a conditional edge
    if s.rows:                                      # @4
        return "answer"                             # @4
    return "rewrite" if s.attempts < 3 else "report"  # @4,5 attempts stops the loop


def run_graph(s, save, node="generate"):
    while True:
        if node == "generate":
            generate(s); node = "run"
        elif node == "run":
            run_query(s); node = after_run(s)       # @3,4
        elif node == "rewrite":
            rewrite(s); node = "run"                # @5
        elif node == "answer":
            answer(s); node = "approve"             # @6
        elif node == "approve":
            if not s.approved:                      # @7 interrupt
                save(s)                             # @7 the state waits in the store
                return "paused for a human"         # @7
            return "END"                            # @8
        elif node == "report":
            return f"report: gave up after {s.attempts} tries"
        save(s)                                     # @9 a checkpoint per node
# panel:end


def main():
    global SERVER_UP
    print("-- tab 1 --")
    saved = []
    s = State(question="0xabc")
    print(" ", run_graph(s, lambda st: saved.append(st.attempts)))
    print(f"  attempts {s.attempts}, rows {len(s.rows)}, draft: {s.draft}, checkpoints {len(saved)}")
    s.approved = True
    print("  human approves:", run_graph(s, lambda st: None, node="approve"))

    print("\n-- tab 2 --")
    SERVER_UP = False
    chat = ChatState()
    lap = 0
    while after_run(chat) == "rewrite" and lap < 7:   # the demo stops itself at lap 7
        lap += 1
    print(f"  chat-only state: lap {lap} and counting, attempts read as {chat.attempts}")
    s = State(question="0xabc")
    copies = []
    print(" ", run_graph(s, lambda st: copies.append(st)), "| last statement:", s.statement)
    print(f"  a token in the state would be saved {len(copies)} times, once per checkpoint: the host holds it instead")


if __name__ == "__main__":
    main()
