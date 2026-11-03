// Server instructions are optional for a host to use. A rule that governs a
// tool also goes into that tool's description. Run: go run .
package main

import (
	"fmt"
	"strings"
)

const rule = "write every address in full"

type Server struct {
	Instructions string // initialize result, field "instructions"
	Description  string // tools/list, the tool's description
}

type Host struct {
	Name          string
	UsesInstrText bool
}

// contextFor is what the model gets from one server on this host.
func contextFor(h Host, s Server) string {
	c := s.Description // every host that lists tools shows this
	if h.UsesInstrText {
		c += " " + s.Instructions
	}
	return c
}

func answer(ctx string) string {
	address := "0x18f3a9c2d4e5b6071829304a5b6c7d8e9f00cb24"
	if strings.Contains(ctx, rule) {
		return address
	}
	return address[:4] + ".." + address[len(address)-4:] // shortened, and nothing fails
}

func main() {
	hosts := []Host{{"Claude Code", true}, {"Another host", false}}
	before := Server{Instructions: rule, Description: "Read the graph."}
	after := Server{Instructions: rule, Description: "Read the graph. Rule: " + rule + "."}
	for _, h := range hosts {
		fmt.Printf("only in instructions, %s: %s\n", h.Name, answer(contextFor(h, before)))
	}
	for _, h := range hosts {
		fmt.Printf("also in the description, %s: %s\n", h.Name, answer(contextFor(h, after)))
	}
}
