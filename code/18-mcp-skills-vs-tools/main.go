// A skill loads on demand in hosts that support it. A tool description reaches
// every host on every request. So a rule goes in both. Run: go run .
package main

import (
	"fmt"
	"strings"
)

type Skill struct{ Name, Description, Body string }

type Tool struct{ Name, Description string }

type Host struct {
	Name       string
	LoadsSkill bool
}

var skill = Skill{"chain-insights-dex", "Use when a question names an exchange", "Recipe: call meta_dex_lookup first, then trace from the pool it returns."}

// context builds what the model sees for one question.
func context(h Host, q string, tools []Tool) string {
	var c []string
	for _, t := range tools { // tools/list reaches every host, every request
		c = append(c, t.Name+": "+t.Description)
	}
	if h.LoadsSkill {
		c = append(c, skill.Name+": "+skill.Description) // name + description only
		if strings.Contains(q, "Aerodrome") {            // the task fits: load the body now
			c = append(c, skill.Body)
		}
	}
	return strings.Join(c, "\n")
}

// model calls the door if any text it saw names it. Otherwise it guesses.
func model(ctx string) string {
	if strings.Contains(ctx, "call meta_dex_lookup first") {
		return "meta_dex_lookup(Aerodrome) -> graph_query(0x8f..2c11, the real pool)"
	}
	return "graph_query(0xdead..beef, a guessed pool) -> wrong money, no error"
}

func main() {
	q := "who sold into an Aerodrome pool?"
	code := Host{"Claude Code", true}
	chat := Host{"Desktop chat", false}
	withRule := []Tool{{"meta_dex_lookup", "Find an exchange. Rule: call meta_dex_lookup first."}}
	noRule := []Tool{{"meta_dex_lookup", "Find an exchange."}}

	fmt.Println("0 at start, skill body in context:", strings.Contains(context(code, "hello", withRule), skill.Body))
	fmt.Println("2 task fits, skill body in context:", strings.Contains(context(code, q, withRule), skill.Body))
	fmt.Println("3", code.Name, "->", model(context(code, q, withRule)))
	fmt.Println("3", chat.Name, "->", model(context(chat, q, withRule)))
	fmt.Println("4 rule only in the skill:")
	fmt.Println("  ", code.Name, "->", model(context(code, q, noRule)))
	fmt.Println("  ", chat.Name, "->", model(context(chat, q, noRule)))
}
