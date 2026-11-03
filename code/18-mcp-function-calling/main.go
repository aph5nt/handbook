// Function calling: the model returns a call, the app runs it. An MCP host does
// the same, and fills the tool list from tools/list. A fake model stands in for
// the model API. Run: go run .
package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type Message struct {
	ToolCall *Call  `json:"tool_call,omitempty"`
	Text     string `json:"text,omitempty"`
}

type Call struct {
	Name  string            `json:"name"`
	Input map[string]string `json:"input"`
}

// fakeModel: with no result yet it returns a call to the tool whose description
// matches the question. With a result it writes the answer.
func fakeModel(question string, tools []ToolDef, result *float64) Message {
	if result != nil {
		return Message{Text: fmt.Sprintf("The balance is %.1f", *result)}
	}
	for _, t := range tools {
		if strings.Contains(strings.ToLower(t.Description), "balance") && strings.Contains(question, "balance") {
			return Message{ToolCall: &Call{t.Name, map[string]string{"address": "0x18..cb24"}}}
		}
	}
	return Message{Text: "I have no tool for that, so I guess: 100"}
}

func getBalance(address string) float64 { return 12.5 }

var balanceTool = ToolDef{"get_balance", "Balance of one address", map[string]any{"type": "object", "required": []string{"address"}}}

// loop is the same in both tabs. Only run() differs: local code or tools/call.
func loop(q string, tools []ToolDef, run func(Call) float64) string {
	m := fakeModel(q, tools, nil)
	if m.ToolCall == nil {
		return m.Text
	}
	b, _ := json.Marshal(m.ToolCall)
	fmt.Println("  model returns:", string(b))
	r := run(*m.ToolCall)
	fmt.Println("  tool result:", r)
	return fakeModel(q, tools, &r).Text
}

// MCP side: tools/list gives name, description, inputSchema.
func toolsList() []ToolDef { return []ToolDef{balanceTool} }

func toolsCall(c Call) float64 {
	fmt.Println("  -> tools/call", c.Name)
	return getBalance(c.Input["address"])
}

func main() {
	q := "what is the balance of this wallet?"
	fmt.Println("plain:")
	fmt.Println(" ", loop(q, []ToolDef{balanceTool}, func(c Call) float64 { return getBalance(c.Input["address"]) }))
	fmt.Println("mcp:")
	fmt.Println(" ", loop(q, toolsList(), toolsCall))
	vague := toolsList()
	vague[0].Description = "Does account stuff"
	fmt.Println("vague description, copied as is:")
	fmt.Println(" ", loop(q, vague, toolsCall))
}
