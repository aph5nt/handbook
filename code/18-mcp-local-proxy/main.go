// A local proxy in front of a remote lane: heavy reads go remote with the
// user's token, lookups are answered locally from public files with a
// packaged fallback. No network: fetchers are functions. Run: go run .
package main

import (
	"encoding/json"
	"errors"
	"fmt"
)

type Exchange struct {
	Name  string   `json:"name"`
	Pools []string `json:"pools"`
}

const packaged = `[{"name":"Aerodrome","pools":["0x8f..2c11"]}]` // shipped in the package

type Proxy struct {
	token   string
	fetch   func() ([]byte, error) // public JSON file in the repository
	lane    func(q, token string) string
	cache   []Exchange
	fromPkg bool
}

func (p *Proxy) load() {
	b, err := p.fetch()
	if err != nil { // unreachable: fall back to the packaged copy
		b, p.fromPkg = []byte(packaged), true
	}
	json.Unmarshal(b, &p.cache)
}

// call routes one tools/call: lookups stay local, graph reads go remote.
func (p *Proxy) call(tool, arg string) string {
	switch tool {
	case "meta_dex_lookup":
		for _, e := range p.cache {
			if e.Name == arg {
				return fmt.Sprintf("%s pools %v (local, packaged=%v)", e.Name, e.Pools, p.fromPkg)
			}
		}
		return "no such exchange"
	case "graph_query":
		return p.lane("anchored on "+arg, p.token)
	}
	return "unknown tool"
}

func main() {
	lane := func(q, tok string) string { return "lane rows (" + q + ", token " + tok + ")" }
	online := func() ([]byte, error) {
		return []byte(`[{"name":"Aerodrome","pools":["0x8f..2c11","0x3a..9e40"]}]`), nil
	}
	p := &Proxy{token: "user-token", fetch: online, lane: lane}
	fmt.Println("0 tools/list over stdio: graph_query, meta_dex_lookup")
	fmt.Println("1", p.call("graph_query", "0x18..cb24"))
	p.load()
	fmt.Println("2 fetched and cached", len(p.cache), "exchange")
	fmt.Println("3", p.call("meta_dex_lookup", "Aerodrome"))

	off := &Proxy{fetch: func() ([]byte, error) { return nil, errors.New("unreachable") }, lane: lane}
	off.load()
	fmt.Println("4", off.call("meta_dex_lookup", "Aerodrome"))
	fmt.Println("5 a table nobody reads: no tool reaches it ->", off.call("find_exchange", "Aerodrome"))
}
