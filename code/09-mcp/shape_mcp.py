"""Official MCP Python SDK shape only. Not run: it needs the mcp package.
The runnable version with the same method names is main.py."""
from mcp.server.fastmcp import FastMCP

mcp = FastMCP("chain-insights")                # @4 the SDK answers initialize


@mcp.tool()                                    # @5 listed by tools/list
def graph_query(statement: str, network: str) -> str:
    """Run an anchored graph query. Write every address in full, 42 characters."""  # @5 the description, in its fixed form with the rule
    return run_on_lane(statement, network)     # @8 runs on tools/call


@mcp.resource("schema://graph")                # @6
def graph_schema() -> str:
    return load_schema_text()                  # @6


@mcp.prompt()                                  # @7
def screen_address(address: str) -> str:
    return f"Screen {address} and name its funders."  # @7


if __name__ == "__main__":
    mcp.run(transport="stdio")                 # @1,3 the host starts this process
