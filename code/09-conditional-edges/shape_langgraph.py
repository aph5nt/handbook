"""LangGraph shape only. Not run: it needs the langgraph package.
The runnable version of the same steps is main.py."""
from langgraph.graph import StateGraph, END

graph = StateGraph(ScreenState)
graph.add_node("score", score)                   # @1
graph.add_node("approve", approve)
graph.add_node("human review", review)
graph.add_node("block", block)
graph.set_entry_point("score")


def route(state: ScreenState) -> str:            # @2
    if state["risk_score"] < 0.3:                # @4
        return "approve"
    if state["risk_score"] <= 0.7:               # @5
        return "human review"
    return "block"                               # @6


graph.add_conditional_edges(                     # @8
    "score",
    route,
    {"approve": "approve",                       # @7 the path map
     "human review": "human review",             # @7
     "block": "block"},                          # @7
)
for node in ["approve", "human review", "block"]:
    graph.add_edge(node, END)                    # @10
app = graph.compile()                            # @7 checks every target node exists
