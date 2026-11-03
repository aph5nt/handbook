// One host, one client per server. Each client owns one connection, and a
// server sees only its own client. Run: go run .
package main

import "fmt"

type Entry struct{ Name, Transport string }

// Server answers what it is asked. stateful=true is the bad server.
type Server struct {
	name     string
	stateful bool
	current  string // (bug) "the current investigation", shared by every client
}

func (s *Server) Call(clientID, q string) string {
	if q == "next hop" {
		if s.stateful {
			return "next hop of " + s.current
		}
		return "error: name the address"
	}
	if s.stateful {
		s.current = clientID + "'s case " + q
	}
	return "rows for " + q
}

// Client holds exactly one connection to exactly one server.
type Client struct {
	id  string
	srv *Server
}

func (c *Client) Ask(q string) string { return c.srv.Call(c.id, q) }

func startHost(name string, cfg []Entry, servers map[string]*Server) map[string]*Client {
	clients := map[string]*Client{}
	for _, e := range cfg { // one client per config entry
		clients[e.Name] = &Client{id: name, srv: servers[e.Name]}
		fmt.Printf("  %s: client for %s over %s\n", name, e.Name, e.Transport)
	}
	return clients
}

func run(stateful bool) {
	servers := map[string]*Server{"filesystem": {name: "filesystem"}, "graph": {name: "graph", stateful: stateful}, "crm": {name: "crm"}}
	code := startHost("Claude Code", []Entry{{"filesystem", "stdio"}, {"graph", "stdio"}, {"crm", "Streamable HTTP"}}, servers)
	desk := startHost("Claude Desktop", []Entry{{"graph", "stdio"}}, servers)
	fmt.Println("  Code ->", code["graph"].Ask("0x18..cb24"))
	fmt.Println("  Desktop ->", desk["graph"].Ask("0x18..cb24"))
	desk["graph"].Ask("0x77..30ff") // Desktop starts its own case
	code["graph"].Ask("0x18..cb24") // then Code asks again
	fmt.Println("  Desktop follow-up ->", desk["graph"].Ask("next hop"))
}

func main() {
	fmt.Println("stateless server:")
	run(false)
	fmt.Println("server that stores the current case:")
	run(true)
}
