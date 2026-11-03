// Four layers, one question: A2A between agents, MCP between host and tools,
// function calling inside one model request, OpenAPI under the server.
// Run: go run .
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

type MCPTool struct{ Name, Description string }

// Function-calling format of a model API: the host converts MCP tools to it.
type Function struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

func toFunctions(ts []MCPTool) []Function {
	var fs []Function
	for _, t := range ts {
		fs = append(fs, Function{t.Name, t.Description, map[string]any{"type": "object"}})
	}
	return fs
}

// The model picks the function whose description shares most words with
// the question. On a tie the later one wins, as a crowded list tends to.
func modelPick(q string, fs []Function) string {
	best, score := "", -1
	for _, f := range fs {
		n := 0
		for _, w := range strings.Fields(q) {
			if strings.Contains(strings.ToLower(f.Description), w) {
				n++
			}
		}
		if n >= score {
			best, score = f.Name, n
		}
	}
	return best
}

// The graph API the MCP server wraps. Its OpenAPI spec describes /flows.
var graphAPI = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, `[{"to":"0x9f..0071","usd":183}]`)
})

func mcpCall(tool string) string { // tools/call -> one or more HTTP calls
	rec := httptest.NewRecorder()
	graphAPI(rec, httptest.NewRequest("GET", "/flows?from=0x18..cb24", nil))
	return tool + " -> GET /flows -> " + rec.Body.String()
}

func main() {
	q := "trace where this wallet money went"
	fmt.Println("layers 0 A2A task from the pricing agent:", q)
	six := []MCPTool{{"trace_funds", "trace where money went from a wallet"}, {"lookup_address", "facts about one address"},
		{"dex_lookup", "find an exchange"}, {"price_at", "token price on a day"}, {"block_at", "block at a time"}, {"schema", "graph labels"}}
	fmt.Println("layers 1 tools/list:", len(six), "tools")
	fs := toFunctions(six)
	b, _ := json.Marshal(fs[0])
	fmt.Println("layers 3 function schema:", string(b))
	pick := modelPick(q, fs)
	fmt.Println("layers 3 model returns call:", pick)
	fmt.Println("layers 4", mcpCall(pick))
	fmt.Println("layers 5 A2A task completed")

	var flood []MCPTool // one tool per OpenAPI operation
	for i := 0; i < 197; i++ {
		flood = append(flood, MCPTool{fmt.Sprintf("op%03d", i), "an operation"})
	}
	flood = append(flood, MCPTool{"getFlows", "where money went"}, MCPTool{"getFlowsV2", "where money went"},
		MCPTool{"getFlowsV2Legacy", "where money went (old unit: wei)"})
	fmt.Println("flood 0 generated:", len(flood), "tools")
	fmt.Println("flood 2 model picks:", modelPick(q, toFunctions(flood)))
	fmt.Println("flood 4 pruned to", len(six), "-> model picks:", modelPick(q, fs))
}
