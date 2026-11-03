// Open host service: the graph offers one versioned protocol (an MCP tool
// server) that every agent host uses the same way.
package main

import (
	"fmt"
	"sort"
)

// Server is the graph's one door. Tools are listed per protocol version.
type Server struct {
	Version int
	Tools   map[string]bool
	Private map[string]string // host -> private endpoint (the failure)
	Storage string
}

// Connect lets any host in that speaks the protocol. No upstream work.
func (s *Server) Connect(host string) string {
	return fmt.Sprintf("%s connected, protocol v%d, %d tools", host, s.Version, len(s.Tools))
}

// Call runs a tool. The host never sees the storage engine.
func (s *Server) Call(host, tool string) string {
	if ep, ok := s.Private[host]; ok {
		return host + " uses private " + ep
	}
	if !s.Tools[tool] {
		return host + ": no tool " + tool
	}
	return host + ": " + tool + " ok"
}

// Doors counts what the graph team must change for every inside change.
func (s *Server) Doors() int { return 1 + len(s.Private) }

// AddTool grows the shared protocol: a new version, for every host.
func (s *Server) AddTool(tool string) {
	s.Tools[tool] = true
	s.Version++
}

func main() {
	s := &Server{Version: 1, Tools: map[string]bool{"graph_query": true, "meta_schema": true}, Private: map[string]string{}, Storage: "engine A"}
	hosts := []string{"Claude Code", "Claude Desktop", "Codex", "cia"}
	for _, h := range hosts {
		fmt.Println(s.Connect(h))
	}
	fmt.Println(s.Connect("fifth host"), "- graph team changed nothing")
	s.Storage = "engine B"
	fmt.Println("storage", s.Storage+":", s.Call("Codex", "graph_query"))
	fmt.Println(s.Call("Codex", "graph_query_batch"))
	s.Private["Codex"] = "/codex/batch" // the failure: a fork
	fmt.Println("doors to change per inside change:", s.Doors())
	delete(s.Private, "Codex") // the fix: grow the protocol instead
	s.AddTool("graph_query_batch")
	names := []string{}
	for t := range s.Tools {
		names = append(names, t)
	}
	sort.Strings(names)
	fmt.Println("v", s.Version, names, "doors:", s.Doors())
	fmt.Println(s.Call("Codex", "graph_query_batch"))
}
