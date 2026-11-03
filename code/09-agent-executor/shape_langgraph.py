"""LangGraph shape of the same executor. Shape only: not run, needs LangGraph."""
from langchain_core.tools import tool
from langgraph.prebuilt import ToolNode, create_react_agent


@tool
def graph_query(address: str) -> str:
    """Rows of money flows for one address."""
    rows = run_query(address)  # @5
    return rows[:4000] + " [trimmed, ask for a narrower query]"  # @7 trim inside the tool


tools = ToolNode([graph_query, lookup], handle_tool_errors=True)  # @4,6 errors become results
agent = create_react_agent(model, tools)  # @2,3,8 the loop: model, tools, append

result = agent.invoke(  # @1
    {"messages": [("user", question)]},
    config={"recursion_limit": 17},  # @1 8 turns, two graph steps each, plus one
)  # GraphRecursionError at the limit: catch it, ask for a partial answer  # @9
