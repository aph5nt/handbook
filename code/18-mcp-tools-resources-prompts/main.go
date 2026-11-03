// Tools, resources and prompts: who pulls each one, and where a rule must live.
// The server lists three kinds of thing. The model calls tools, the host app
// attaches resources, the user starts prompts. Run: go run .
package main

import (
	"fmt"
	"strings"
)

type Tool struct {
	Name, Description string
	ReadOnly, Destroy bool
}

type Server struct {
	Tools     []Tool
	Resources map[string]string // uri -> contents
	Prompts   map[string]string // name -> messages
	subs      []func(uri string)
}

func (s *Server) ReadResource(uri string) string { return s.Resources[uri] }
func (s *Server) GetPrompt(name string) string   { return s.Prompts[name] }
func (s *Server) Subscribe(f func(string))       { s.subs = append(s.subs, f) }
func (s *Server) ChangeResource(uri, v string) {
	s.Resources[uri] = v
	for _, f := range s.subs {
		f(uri) // notifications/resources/updated
	}
}

// Host runs a tool call the model chose. Consent comes from the user.
func callTool(t Tool, ask func(string) bool) string {
	if t.Destroy && !ask("Allow "+t.Name+"?") {
		return "denied by user"
	}
	return "ran " + t.Name
}

// Model sees only what the host put in its context.
// It anchors the search only if it has seen the rule.
func modelQuery(context string) string {
	if strings.Contains(context, "anchor every search") {
		return "graph_query anchored on 0x18..cb24 -> rows"
	}
	return "graph_query, no anchor -> refused"
}

func toolList(s *Server) string {
	var b strings.Builder
	for _, t := range s.Tools {
		fmt.Fprintf(&b, "%s: %s\n", t.Name, t.Description)
	}
	return b.String()
}

func main() {
	s := &Server{
		Tools: []Tool{
			{Name: "graph_query", Description: "Read the graph.", ReadOnly: true},
			{Name: "wallet_topup", Description: "Spend tokens.", Destroy: true},
		},
		Resources: map[string]string{"schema://robinhood": "labels: Address, FLOWS_TO"},
		Prompts:   map[string]string{"trace-funds": "user: trace 0x18..cb24. Rule: anchor every search."},
	}
	fmt.Printf("1 list: %d tools, %d resource, %d prompt\n", len(s.Tools), len(s.Resources), len(s.Prompts))

	// Resource: the host decides to attach it. The model never reads it itself.
	ctx := s.ReadResource("schema://robinhood")
	fmt.Println("2 host attached resource:", ctx)

	// Tool: the model decides. A read-only hint lets the host run it at once.
	fmt.Println("3 model -> graph_query:", callTool(s.Tools[0], nil))
	fmt.Println("4 model -> wallet_topup:", callTool(s.Tools[1], func(q string) bool {
		fmt.Println("  host asks user:", q, "-> approve")
		return true
	}))

	// Prompt: the user decides by typing a slash command.
	fmt.Println("5 user /trace-funds -> prompts/get:", s.GetPrompt("trace-funds"))

	s.Subscribe(func(uri string) { ctx = s.ReadResource(uri); fmt.Println("6 updated", uri, "-> read again:", ctx) })
	s.ChangeResource("schema://robinhood", "labels: Address, FLOWS_TO, SWAPPED")

	// The failure: the rule lives only in the prompt.
	fmt.Println("hidden 2 user A (typed /trace-funds):", modelQuery(s.GetPrompt("trace-funds")))
	fmt.Println("hidden 3 user B (plain words):", modelQuery("who sent the most today?"))
	s.Resources["rules://graph"] = "anchor every search on one address"
	fmt.Println("hidden 4 rule in a resource, not attached:", modelQuery("who sent the most today?"))

	// The fix: the rule goes into the tool description, which every host shows.
	s.Tools[0].Description = "Read the graph. Rule: anchor every search on one address."
	fmt.Print("hidden 5 tools/list:\n", toolList(s))
	fmt.Println("hidden 6 user B again:", modelQuery(toolList(s)+"who sent the most today?"))
}
