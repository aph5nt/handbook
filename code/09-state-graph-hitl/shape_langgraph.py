"""LangGraph shape only. Not run: it needs the langgraph package.
The runnable version of the same steps is main.py."""
from typing import TypedDict
from langgraph.graph import StateGraph, END
from langgraph.checkpoint.postgres import PostgresSaver
from langgraph.types import interrupt


class State(TypedDict):                        # @1 typed fields, not chat text
    statement: str
    rows: list
    reason: str
    attempts: int                              # @1


def approve(state):
    interrupt({"draft": state["draft"]})       # @7 the run stops here


def after_run(state) -> str:                   # @4
    if state["rows"]:
        return "answer"
    return "rewrite" if state["attempts"] < 3 else "report"  # @4


g = StateGraph(State)
g.add_node("generate", generate)               # @2
g.add_node("run", run_query)                   # @3
g.add_node("rewrite", rewrite)                 # @5
g.add_node("answer", answer)                   # @6
g.add_node("approve", approve)                 # @7
g.set_entry_point("generate")
g.add_edge("generate", "run")
g.add_conditional_edges("run", after_run)      # @4
g.add_edge("rewrite", "run")                   # @5
g.add_edge("answer", "approve")
g.add_edge("approve", END)                     # @8
app = g.compile(checkpointer=PostgresSaver.from_conn_string(DB_URL))  # @9
