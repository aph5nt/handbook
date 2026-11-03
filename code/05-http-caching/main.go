// HTTP caching over a real origin server: a browser, a CDN edge and the origin.
// A sealed day: max-age, ETag and 304. Then a personal page marked public.
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
)

const day = 86400

type copyOf struct {
	body, etag string
	maxAge     int
	stored     int
	public     bool
}

type stats struct{ originBodies, origin304 int }

// fetch does one conditional GET against the origin.
func fetch(url, user, inm string, st *stats) (body, etag string, maxAge int, public bool, notModified bool) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-User", user)
	if inm != "" {
		req.Header.Set("If-None-Match", inm)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	cc := resp.Header.Get("Cache-Control")
	for _, d := range strings.Split(cc, ",") {
		if name, val, ok := strings.Cut(strings.TrimSpace(d), "="); ok && name == "max-age" {
			maxAge, _ = strconv.Atoi(val)
		}
	}
	public = strings.Contains(cc, "public")
	if strings.Contains(cc, "no-store") { // nobody keeps it
		maxAge, public = 0, false
	}
	if strings.Contains(cc, "no-cache") { // keep it, but check before every use
		maxAge = 0
	}
	if resp.StatusCode == http.StatusNotModified {
		st.origin304++
		return "", resp.Header.Get("ETag"), maxAge, public, true
	}
	st.originBodies++
	return string(b), resp.Header.Get("ETag"), maxAge, public, false
}

// Cache is one layer. The edge and the browser use the same rule.
type Cache struct {
	store map[string]copyOf
	name  string
}

// lookup reports a copy and whether it is still fresh.
func (c *Cache) lookup(key string, now int) (copyOf, bool, bool) {
	e, ok := c.store[key]
	return e, ok, ok && now-e.stored < e.maxAge
}

type World struct {
	origin  string
	edge    *Cache
	st      stats
	now     int
	network int // requests that left the browser
}

func (w *World) Get(browser *Cache, path, user string) (string, string) {
	if e, _, fresh := browser.lookup(path, w.now); fresh {
		return e.body, "browser cache, no request"
	}
	w.network++
	if e, ok, fresh := w.edge.lookup(path, w.now); ok && fresh && e.public {
		browser.store[path] = e
		return e.body, "edge hit, origin idle"
	}
	inm := ""
	old, had, _ := w.edge.lookup(path, w.now)
	if had {
		inm = old.etag
	}
	body, etag, maxAge, public, nm := fetch(w.origin+path, user, inm, &w.st)
	how := "origin 200"
	if nm {
		body, etag, how = old.body, old.etag, "origin 304, no body"
	}
	e := copyOf{body, etag, maxAge, w.now, public}
	if public { // a shared cache keeps only what is public
		w.edge.store[path] = e
	}
	if maxAge > 0 {
		browser.store[path] = e
	}
	return body, how
}

func main() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("ETag", `"v7"`)
		if r.Header.Get("If-None-Match") == `"v7"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		io.WriteString(w, "facts of the sealed day")
	}))
	defer srv.Close()

	fmt.Println("== a sealed day: public, max-age=86400, ETag v7 ==")
	w := &World{origin: srv.URL, edge: &Cache{map[string]copyOf{}, "edge"}}
	b1, b2 := &Cache{map[string]copyOf{}, "b1"}, &Cache{map[string]copyOf{}, "b2"}
	step := func(label string, b *Cache) {
		_, how := w.Get(b, "/day", "")
		fmt.Printf("  %-34s %-26s origin bodies=%d 304s=%d\n", label, how, w.st.originBodies, w.st.origin304)
	}
	step("first visit", b1)
	w.now = 3600
	step("reload an hour later", b1)
	step("second user, same city", b2)
	w.now = day + 1
	step("after a day", b1)

	fmt.Println("== the leak: /me marked public, max-age=300 ==")
	leak := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Cache-Control", "public, max-age=300")
		io.WriteString(rw, "account of "+r.Header.Get("X-User"))
	}))
	defer leak.Close()
	w = &World{origin: leak.URL, edge: &Cache{map[string]copyOf{}, "edge"}}
	a, how := w.Get(&Cache{map[string]copyOf{}, "alice"}, "/me", "Alice")
	fmt.Printf("  Alice sees %q (%s)\n", a, how)
	bb, how := w.Get(&Cache{map[string]copyOf{}, "bob"}, "/me", "Bob")
	fmt.Printf("  Bob sees %q (%s)\n", bb, how)

	fmt.Println("== the fix: Cache-Control: private ==")
	fixed := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Cache-Control", "private, max-age=300")
		io.WriteString(rw, "account of "+r.Header.Get("X-User"))
	}))
	defer fixed.Close()
	w = &World{origin: fixed.URL, edge: &Cache{map[string]copyOf{}, "edge"}}
	a, _ = w.Get(&Cache{map[string]copyOf{}, "alice"}, "/me", "Alice")
	bb, how = w.Get(&Cache{map[string]copyOf{}, "bob"}, "/me", "Bob")
	fmt.Printf("  Alice sees %q. Bob sees %q (%s)\n", a, bb, how)
}
