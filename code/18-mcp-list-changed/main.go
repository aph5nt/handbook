// listChanged: the server declares it, then sends a bare notification when
// its tools change. The host lists again. Run: go run .
package main

import "fmt"

type Server struct {
	listChanged bool
	tools       []string
	notify      func(method string)
}

func (s *Server) Initialize() map[string]any {
	return map[string]any{"tools": map[string]bool{"listChanged": s.listChanged}}
}

func (s *Server) Unlock(names ...string) {
	s.tools = append(s.tools, names...)
	if s.listChanged {
		s.notify("notifications/tools/list_changed") // news only, no list
	}
}

type Host struct {
	srv   *Server
	shown []string // what the model sees
}

func (h *Host) list() {
	h.shown = append([]string(nil), h.srv.tools...)
	fmt.Println("  tools/list ->", len(h.shown), "tools shown to the model")
}

func (h *Host) onNotify(method string) {
	if method == "notifications/tools/list_changed" {
		h.list() // ask again, and re-check pinned descriptions
	}
}

func run(listChanged bool) {
	s := &Server{listChanged: listChanged, tools: []string{"graph_query", "graph_query_batch", "meta_schema", "meta_rules", "wallet_balance", "meta_help"}}
	h := &Host{srv: s}
	s.notify = h.onNotify
	fmt.Println("0 initialize ->", s.Initialize())
	h.list()
	fmt.Println("2 top-up: 2 paid tools unlock")
	s.Unlock("aml_address_risk", "meta_dex_lookup")
	fmt.Println("server has", len(s.tools), "tools, model sees", len(h.shown))
}

func main() {
	run(true)
	fmt.Println("-- a server that never declared listChanged")
	run(false)
}
