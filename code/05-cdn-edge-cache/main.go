// A CDN as a model: edges near users, one optional origin shield, and the
// origin in Frankfurt. Three regions, four users, one origin request. Then a
// global purge, with and without the shield, and hashed names with no purge.
package main

import "fmt"

type Origin struct{ requests int }

func (o *Origin) Fetch(url string) string {
	o.requests++
	return "body of " + url
}

// Shield is one extra cache layer between the edges and the origin.
type Shield struct {
	store  map[string]string
	origin *Origin
}

func (s *Shield) Fetch(url string) string {
	if v, ok := s.store[url]; ok {
		return v // many edges, one origin request per file
	}
	v := s.origin.Fetch(url)
	s.store[url] = v
	return v
}

type fetcher interface{ Fetch(string) string }

// Edge answers from nearby, or asks upstream on a miss.
type Edge struct {
	name     string
	store    map[string]string
	upstream fetcher
}

func (e *Edge) Get(url string) (string, string) {
	if v, ok := e.store[url]; ok {
		return v, "hit"
	}
	v := e.upstream.Fetch(url)
	e.store[url] = v
	return v, "miss"
}

func (e *Edge) Purge() { e.store = map[string]string{} }

func edges(names []string, up fetcher) []*Edge {
	var out []*Edge
	for _, n := range names {
		out = append(out, &Edge{n, map[string]string{}, up})
	}
	return out
}

func main() {
	fmt.Println("== edges and a shield ==")
	o := &Origin{}
	sh := &Shield{map[string]string{}, o}
	es := edges([]string{"Warsaw", "Sao Paulo", "Singapore"}, sh)
	const day = "/facts/2026-10-06"
	for _, u := range []struct {
		who  string
		edge *Edge
	}{{"user 1 Warsaw", es[0]}, {"user 2 Warsaw", es[0]}, {"user Sao Paulo", es[1]}, {"user Singapore", es[2]}} {
		_, r := u.edge.Get(day)
		fmt.Printf("  %-15s %s edge: %-4s origin requests so far: %d\n", u.who, u.edge.name, r, o.requests)
	}

	const pops = 300 // a real CDN has hundreds of edges
	names := make([]string, pops)
	for i := range names {
		names[i] = fmt.Sprint("edge-", i)
	}
	warm := func(list []*Edge, url string) {
		for _, e := range list {
			e.Get(url)
		}
	}
	purgeAll := func(list []*Edge) {
		for _, e := range list {
			e.Purge()
		}
	}
	fmt.Printf("== a global purge, %d edges, one file ==\n", pops)
	o = &Origin{}
	list := edges(names, o) // no shield
	warm(list, "/app.js")
	o.requests = 0
	purgeAll(list)
	warm(list, "/app.js")
	fmt.Printf("  no shield:   %d origin requests\n", o.requests)

	o = &Origin{}
	sh = &Shield{map[string]string{}, o}
	list = edges(names, sh)
	warm(list, "/app.js")
	o.requests = 0
	purgeAll(list)
	sh.store = map[string]string{} // the purge reaches the shield too
	warm(list, "/app.js")
	fmt.Printf("  with shield: %d origin request\n", o.requests)

	o = &Origin{}
	sh = &Shield{map[string]string{}, o}
	list = edges(names, sh)
	warm(list, "/app.3f9a.js")
	o.requests = 0
	warm(list, "/app.3f9a.js") // users on the old page: copies stay valid
	fmt.Printf("  hashed names, no purge: old file %d origin requests, ", o.requests)
	warm(list, "/app.7c21.js") // the new name is fetched on demand
	fmt.Printf("new file %d\n", o.requests)
}
