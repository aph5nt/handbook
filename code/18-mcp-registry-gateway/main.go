// Registry, mirror, gateway: only reviewed servers at a pinned version are
// reachable, every call is logged. Run: go run .
package main

import (
	"encoding/json"
	"fmt"
)

// ServerJSON is one registry entry.
type ServerJSON struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Package string `json:"package"`
}

type Gateway struct {
	mirror map[string]string // server name -> reviewed version
	log    []string
}

func (g *Gateway) review(e ServerJSON) { g.mirror[e.Name] = e.Version }

func (g *Gateway) call(user, server, version, tool string) string {
	if user == "" {
		return "401 sign in first"
	}
	want, ok := g.mirror[server]
	if !ok {
		return "403 " + server + " is not in the mirror"
	}
	if version != want {
		return "403 " + server + " " + version + " is not the reviewed " + want
	}
	g.log = append(g.log, user+" "+server+" "+tool)
	return "200 result from " + server + " " + version
}

func main() {
	public := []ServerJSON{
		{"io.github.acme/github", "1.4.2", "npm:@acme/github"},
		{"io.github.acme/github", "1.5.0", "npm:@acme/github"},
		{"io.github.mallory/github-official", "9.9.9", "npm:github-officiall"},
	}
	b, _ := json.Marshal(public[0])
	fmt.Println("0 registry entry:", string(b), "of", len(public))

	g := &Gateway{mirror: map[string]string{}}
	g.review(public[0])
	fmt.Println("1 reviewed and mirrored:", g.mirror)
	fmt.Println("3", g.call("dev@acme", "io.github.acme/github", "1.4.2", "read_issue"))
	fmt.Println("  audit log:", g.log)
	fmt.Println("3b", g.call("dev@acme", "io.github.acme/github", "1.5.0", "read_issue"))
	fmt.Println("4 through the gateway:", g.call("dev@acme", public[2].Name, public[2].Version, "read_issue"))
	fmt.Println("4 installed straight from the public list: no review, no log, no gate. Log lines:", len(g.log))
}
