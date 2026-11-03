// CDN and edge: the story of the lesson 12-cdn-and-edge.html, replayed.
// Edges cache pages from one origin with a TTL, serve stale while one request
// refreshes, run a redirect at the edge, then a purge-all stampedes the origin
// and a page without a private header leaks one user's data.
package main

import "fmt"

// Response is what the origin sends back.
type Response struct {
	Body    string
	TTL     int  // seconds, from Cache-Control: max-age
	Private bool // Cache-Control: private
}

// Origin is the one real server. Calls counts every request it gets.
type Origin struct {
	Calls int
	Pages func(path, user string) Response
}

// Entry is one cached copy at an edge.
type Entry struct {
	Resp       Response
	Expires    int
	Refreshing bool
}

// Edge is one CDN location, such as Frankfurt or Tokyo.
type Edge struct {
	Name  string
	Cache map[string]*Entry
	O     *Origin
}

// Get answers one request at time now.
func (e *Edge) Get(path, user string, now int) (string, string) {
	if path == "/help" {
		return "301 to /docs", "edge function" // code at the edge, no origin call
	}
	if c, ok := e.Cache[path]; ok {
		if now < c.Expires {
			return c.Resp.Body, "hit"
		}
		if !c.Refreshing {
			c.Refreshing = true // one request refreshes, the rest get stale
			e.fetch(path, user, now)
		}
		return c.Resp.Body, "stale"
	}
	r := e.fetch(path, user, now)
	return r.Body, "miss"
}

func (e *Edge) fetch(path, user string, now int) Response {
	e.O.Calls++
	r := e.O.Pages(path, user)
	if !r.Private && r.TTL > 0 {
		e.Cache[path] = &Entry{Resp: r, Expires: now + r.TTL}
	}
	return r
}

// PurgeAll drops every page. Purge drops the listed paths only.
func (e *Edge) PurgeAll() { e.Cache = map[string]*Entry{} }

func (e *Edge) Purge(paths ...string) {
	for _, p := range paths {
		if c, ok := e.Cache[p]; ok {
			c.Expires = 0 // mark stale, keep the copy to serve meanwhile
		}
	}
}

func pages(markPrivate bool) func(string, string) Response {
	return func(path, user string) Response {
		if path == "/account" {
			return Response{Body: user + "'s balance", TTL: 3600, Private: markPrivate}
		}
		return Response{Body: "page " + path, TTL: 3600}
	}
}

func main() {
	fmt.Println("== hit: hit, miss and TTL")
	o := &Origin{Pages: pages(true)}
	fra := &Edge{Name: "Frankfurt", Cache: map[string]*Entry{}, O: o}
	tyo := &Edge{Name: "Tokyo", Cache: map[string]*Entry{}, O: o}
	_, how := fra.Get("/docs", "warsaw 1", 0)
	fmt.Println("1 Warsaw /docs at Frankfurt:", how, " origin calls", o.Calls)
	fmt.Println("2 Frankfurt keeps /docs until t =", fra.Cache["/docs"].Expires, "s")
	_, how = fra.Get("/docs", "warsaw 2", 10)
	fmt.Println("3 next Warsaw user:", how, " origin calls", o.Calls)
	_, how = tyo.Get("/docs", "tokyo 1", 20)
	fmt.Println("4 first Tokyo user:", how, " origin calls", o.Calls)
	hits := 0
	for k := 0; k < 1000000; k++ {
		if _, h := tyo.Get("/docs", "fan", 30+k/400); h == "hit" {
			hits++
		}
	}
	fmt.Println("5 viral: 1,000,000 Tokyo requests,", hits, "hits, origin calls", o.Calls)
	seen := map[string]int{}
	for k := 0; k < 1000; k++ {
		_, h := tyo.Get("/docs", "fan", 3620)
		seen[h]++
	}
	fmt.Printf("6 TTL over at t=3620 s: 1,000 requests, %d stale, %d hits after the refresh, origin calls %d\n",
		seen["stale"], seen["hit"], o.Calls)
	body, how := fra.Get("/help", "warsaw 3", 3700)
	fmt.Println("7 /help:", body, "by the", how, " origin calls", o.Calls)

	fmt.Println("== purge: stampede and a private page")
	const capacity = 2000 // origin requests per minute it can serve
	o = &Origin{Pages: pages(true)}
	edges := []*Edge{}
	for k := 0; k < 10; k++ {
		e := &Edge{Name: fmt.Sprint("edge ", k), Cache: map[string]*Entry{}, O: o}
		for p := 0; p < 600; p++ {
			e.Cache[fmt.Sprint("/p", p)] = &Entry{Resp: Response{Body: "page", TTL: 3600}, Expires: 3600}
		}
		edges = append(edges, e)
	}
	normal := 10 * 600 / 60 // each object refreshes once an hour
	fmt.Printf("1 10 edges x 600 pages, %d refreshes a minute: origin at %d%%\n", normal, normal*100/capacity)
	for _, e := range edges {
		e.PurgeAll()
	}
	o.Calls = 0
	for _, e := range edges {
		for p := 0; p < 600; p++ {
			e.Get(fmt.Sprint("/p", p), "user", 60)
		}
	}
	fmt.Printf("2 purge all, next minute: %d misses reach the origin: %d%% of capacity\n", o.Calls, o.Calls*100/capacity)
	for _, e := range edges {
		for p := 0; p < 600; p++ {
			e.Cache[fmt.Sprint("/p", p)].Expires = 7200
		}
	}
	o.Calls = 0
	for _, e := range edges {
		e.Purge("/p0") // only the changed page
		for k := 0; k < 50; k++ {
			e.Get("/p0", "user", 120)
		}
	}
	fmt.Printf("3 purge /p0 only, 500 requests: %d refreshes, the rest stale: origin at %d%%\n",
		o.Calls, (normal+o.Calls)*100/capacity)

	o = &Origin{Pages: pages(false)} // forgot Cache-Control: private
	fra = &Edge{Name: "Frankfurt", Cache: map[string]*Entry{}, O: o}
	fra.Get("/account", "Ana", 0)
	_, cached := fra.Cache["/account"]
	fmt.Println("4 Ana's /account without a private header, cached:", cached)
	body, how = fra.Get("/account", "Ben", 5)
	fmt.Println("5 Ben opens /account:", how, "->", body)
	o = &Origin{Pages: pages(true)}
	fra = &Edge{Name: "Frankfurt", Cache: map[string]*Entry{}, O: o}
	fra.Get("/account", "Ana", 0)
	body, how = fra.Get("/account", "Ben", 5)
	fmt.Println("  with Cache-Control: private:", how, "->", body)
}
