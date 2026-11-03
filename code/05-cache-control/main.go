// Cache-Control: six responses, six headers. A real HTTP server, and a small
// shared cache (the CDN edge) that obeys the directives. Then the bug: the
// account page sent as public, so the edge shows Alice's page to Bob.
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
)

type Policy struct {
	BrowserKeeps, EdgeKeeps bool
	BrowserTTL, EdgeTTL     int  // seconds
	CheckFirst              bool // no-cache: keep, but ask before every use
	NeverCheck              bool // immutable
}

// Decide reads a Cache-Control header the way a browser and an edge do.
func Decide(h string) Policy {
	p, sMaxAge := Policy{}, -1
	for _, d := range strings.Split(h, ",") {
		name, val, _ := strings.Cut(strings.TrimSpace(d), "=")
		n, _ := strconv.Atoi(val)
		switch name {
		case "max-age":
			p.BrowserTTL, p.EdgeTTL = n, n
			p.BrowserKeeps, p.EdgeKeeps = true, true
		case "s-maxage": // only for shared caches, wins over max-age there
			sMaxAge, p.EdgeKeeps = n, true
		case "immutable":
			p.NeverCheck = true
		case "no-cache":
			p.BrowserKeeps, p.EdgeKeeps, p.CheckFirst = true, true, true
		}
	}
	if sMaxAge >= 0 {
		p.EdgeTTL = sMaxAge
	}
	if strings.Contains(h, "private") { // browser yes, shared caches no
		p.BrowserKeeps, p.EdgeKeeps, p.EdgeTTL = true, false, 0
	}
	if strings.Contains(h, "no-store") { // nobody keeps a copy at all
		return Policy{}
	}
	return p
}

// edge is a shared cache keyed by URL only, as in the failure.
type edge struct {
	origin string
	store  map[string]string
}

func (e *edge) get(path, user string) (string, bool) {
	if body, ok := e.store[path]; ok {
		return body, true
	}
	req, _ := http.NewRequest("GET", e.origin+path, nil)
	req.Header.Set("X-User", user)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if Decide(resp.Header.Get("Cache-Control")).EdgeKeeps {
		e.store[path] = string(b)
	}
	return string(b), false
}

func main() {
	rows := []struct{ what, header string }{
		{"hashed script app.3f9a.js", "public, max-age=31536000, immutable"},
		{"facts of a sealed day", "public, max-age=86400"},
		{"exchange list", "public, max-age=60, s-maxage=600"},
		{"account page", "private, max-age=0"},
		{"payment page", "no-store"},
		{"live chain head", "no-store"},
	}
	fmt.Printf("%-28s %-38s %-14s %s\n", "response", "Cache-Control", "browser", "edge")
	for _, r := range rows {
		p := Decide(r.header)
		fmt.Printf("%-28s %-38s %-14s %s\n", r.what, r.header, keep(p.BrowserKeeps, p.BrowserTTL), keep(p.EdgeKeeps, p.EdgeTTL))
	}
	fmt.Printf("\nno-cache: keeps a copy=%v, must check first=%v. no-store: keeps a copy=%v\n",
		Decide("no-cache").BrowserKeeps, Decide("no-cache").CheckFirst, Decide("no-store").BrowserKeeps)

	for _, header := range []string{"public, max-age=300", "private, max-age=0"} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", header)
			fmt.Fprintf(w, "account of %s", r.Header.Get("X-User"))
		}))
		e := &edge{origin: srv.URL, store: map[string]string{}}
		fmt.Printf("\naccount page sent as %q:\n", header)
		a, _ := e.get("/account", "Alice")
		b, hit := e.get("/account", "Bob")
		fmt.Printf("  Alice sees %q. Bob sees %q (edge hit=%v)\n", a, b, hit)
		srv.Close()
	}
}

func keep(k bool, ttl int) string {
	if !k {
		return "no"
	}
	return fmt.Sprintf("yes, %d s", ttl)
}
