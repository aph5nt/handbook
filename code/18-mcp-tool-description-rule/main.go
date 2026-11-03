// The tool description is the one text every host reads. A rule only in a
// skill misses every host that loads no skill. A contract test keeps the
// door in place. Run: go run .
package main

import (
	"fmt"
	"strings"
)

const rule = "call first when an exchange is named, never guess an address"

type Host struct {
	Name       string
	LoadsSkill bool
}

var hosts = []Host{{"Claude Code", true}, {"Codex", true}, {"Claude Desktop chat", false}, {"cia CLI", false}}

// context is the text one host's model sees.
func context(h Host, description, skillBody string) string {
	c := description // tools/list: every host, or there is no tool at all
	if h.LoadsSkill {
		c += " " + skillBody // only hosts that load skills
	}
	return c
}

func answer(ctx string) string {
	if strings.Contains(ctx, rule) {
		return "meta_dex_lookup(Aerodrome) -> right pools"
	}
	return "guessed address -> wrong money, no error"
}

func report(description, skillBody string) int {
	right := 0
	for _, h := range hosts {
		a := answer(context(h, description, skillBody))
		if strings.Contains(a, "right pools") {
			right++
		}
		fmt.Printf("  %-20s %s\n", h.Name, a)
	}
	return right
}

// contractTest: every skill must have a tool-shaped door whose description
// carries the skill's opening rule.
func contractTest(skills map[string]string, tools map[string]string) []string {
	var fails []string
	for skill, door := range skills {
		desc, ok := tools[door]
		if !ok {
			fails = append(fails, "skill "+skill+": no door "+door)
		} else if !strings.Contains(desc, rule) {
			fails = append(fails, "skill "+skill+": door "+door+" lacks the rule")
		}
	}
	return fails
}

func main() {
	fmt.Println("rule only in a skill:")
	fmt.Println("  right:", report("Find an exchange by name", rule), "of", len(hosts))
	fmt.Println("rule in the description:")
	fmt.Println("  right:", report("Find an exchange by name: "+rule, "recipe and examples"), "of", len(hosts))

	skills := map[string]string{"chain-insights-dex": "meta_dex_lookup"}
	fmt.Println("test, old description:", contractTest(skills, map[string]string{"meta_dex_lookup": "Find an exchange by name"}))
	fmt.Println("test, new description:", contractTest(skills, map[string]string{"meta_dex_lookup": "Find an exchange by name: " + rule}))
	fmt.Println("test, a new skill with no door:", contractTest(map[string]string{"chain-insights-new": "meta_new_lookup"}, map[string]string{"meta_dex_lookup": rule}))
}
