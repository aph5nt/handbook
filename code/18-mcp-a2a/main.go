// A2A (Agent2Agent): an agent card, then a task with states. Inside, our agent
// uses MCP for its own tools. Wrapping a whole agent as one MCP tool loses the
// states and the question back. Run: go run .
package main

import (
	"encoding/json"
	"fmt"
)

type AgentCard struct {
	Name     string   `json:"name"`
	Skills   []string `json:"skills"`
	Endpoint string   `json:"url"`
	Auth     string   `json:"auth"`
}

// The task state machine. Any other move is refused.
var moves = map[string][]string{
	"submitted":      {"working"},
	"working":        {"input-required", "completed", "failed"},
	"input-required": {"working"},
}

type Task struct {
	ID, State string
	Artifact  string
}

func (t *Task) move(to string) error {
	for _, n := range moves[t.State] {
		if n == to {
			fmt.Printf("  task %s: %s -> %s\n", t.ID, t.State, to)
			t.State = to
			return nil
		}
	}
	return fmt.Errorf("task %s: %s -> %s refused", t.ID, t.State, to)
}

func mcpGraphQuery(network string) string { return "rows on " + network }

// run is our agent. ask sends a question back to the calling agent.
func run(t *Task, ask func(string) string) {
	t.move("working")
	fmt.Println("  inside, MCP tools/call graph_query:", mcpGraphQuery("all"))
	t.move("input-required") // the wallet exists on two networks
	network := ask("which network?")
	t.move("working")
	t.Artifact = "trace: " + mcpGraphQuery(network)
	t.move("completed")
}

// wrapped is the same agent behind one MCP tool: one request, one result.
func wrapped() string {
	network := "base" // cannot ask, so it guesses
	return "result after 10 minutes, no state on the way: trace: " + mcpGraphQuery(network)
}

func main() {
	card := AgentCard{"chain-insights", []string{"trace_funds"}, "https://agent.example/a2a", "bearer"}
	b, _ := json.Marshal(card)
	fmt.Println("0 GET /.well-known/agent-card.json:", string(b))
	t := &Task{ID: "t-1", State: "submitted"}
	fmt.Println("1 message: trace 0x18..cb24, state", t.State)
	run(t, func(q string) string { fmt.Println("  asks back:", q, "-> robinhood"); return "robinhood" })
	fmt.Println("4 state", t.State, "artifact", t.Artifact)
	fmt.Println("  a done task cannot move:", t.move("working"))
	fmt.Println("5", wrapped())
}
