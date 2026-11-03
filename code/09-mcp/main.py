"""MCP: one tool server, every host.

Replays visual/09-mcp.html. A tiny server answers the real MCP method names
over JSON text, in-process instead of stdio. No MCP package is needed.
Run: python3 main.py
"""
import json

RULE = "Write every address in full, 42 characters."


class Server:
    def __init__(self, rule_in_description):
        tail = " " + RULE if rule_in_description else ""
        self.tools = [
            {"name": "graph_query", "description": "Run an anchored graph query." + tail,
             "inputSchema": {"type": "object", "properties": {"statement": {"type": "string"}}}},
            {"name": "meta_dex_lookup", "description": "Find an exchange by name.",
             "inputSchema": {"type": "object", "properties": {"name": {"type": "string"}}}},
        ]

    # panel:start
    def handle(self, raw):                                  # @3
        msg = json.loads(raw)                               # @2,3 JSON-RPC over stdio
        method, params = msg["method"], msg.get("params", {})
        if method == "initialize":                          # @4
            result = {"protocolVersion": "2025-06-18",      # @4 both sides say what they support
                      "capabilities": {"tools": {}, "resources": {}, "prompts": {}}}  # @4
        elif method == "tools/list":                        # @5
            result = {"tools": self.tools}                  # @5 names, descriptions, schemas
        elif method == "resources/read":                    # @6
            result = {"contents": [{"uri": params["uri"], "text": "the graph schema"}]}  # @6
        elif method == "prompts/list":                      # @7
            result = {"prompts": [{"name": "screen this address"}]}  # @7
        elif method == "tools/call":                        # @8
            result = {"content": [{"type": "text", "text": f"rows for {params['name']}"}]}  # @8
        else:
            result = {"error": "unknown method"}
        return json.dumps({"id": msg["id"], "result": result})
    # panel:end


class Host:
    def __init__(self, name, server, loads_skills):
        self.name, self.server, self.loads_skills, self.next = name, server, loads_skills, 0

    def call(self, method, **params):
        self.next += 1
        return json.loads(self.server.handle(json.dumps({"id": self.next, "method": method, "params": params})))["result"]

    def knows_rule(self):
        """What the model in this host can read: tool descriptions, plus skills if it loads them."""
        descriptions = " ".join(t["description"] for t in self.call("tools/list")["tools"])
        return RULE in descriptions or self.loads_skills


def main():
    print("-- tab 1 --")
    server = Server(rule_in_description=False)
    for name in ["Claude Desktop", "Claude Code"]:
        h = Host(name, server, loads_skills=False)
        init = h.call("initialize")
        print(f"{name}: initialize", init["protocolVersion"], "capabilities:", list(init["capabilities"]))
        print("  tools:", [t["name"] for t in h.call("tools/list")["tools"]])
    print("  resource:", h.call("resources/read", uri="schema://graph")["contents"][0]["text"])
    print("  prompts:", h.call("prompts/list")["prompts"][0]["name"])
    print("  call:", h.call("tools/call", name="graph_query", arguments={})["content"][0]["text"])

    print("\n-- tab 2 --")
    print("graph_query description:", server.tools[0]["description"])
    print("rule in a skill file: Claude Code knows it:", Host("code", server, True).knows_rule())
    print("rule in a skill file: Desktop chat knows it:", Host("chat", server, False).knows_rule())
    fixed = Server(rule_in_description=True)
    print("fixed description:", fixed.tools[0]["description"])
    print("rule in the description: Desktop chat knows it:", Host("chat", fixed, False).knows_rule())


if __name__ == "__main__":
    main()
