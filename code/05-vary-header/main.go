// Vary over real HTTP. An edge keys its copies by URL plus the headers the
// origin names in Vary. No Vary: the CSV user gets JSON. Vary: User-Agent:
// every browser version gets its own copy, and the hit rate falls.
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
)

type copyOf struct {
	body string
	vary string
}

// Edge keeps one copy per URL plus the values of the Vary headers.
type Edge struct {
	origin       string
	seenVary     map[string]string // url -> the Vary header the origin sent
	store        map[string]copyOf
	hits, misses int
}

func key(url, vary string, h http.Header) string {
	k := url
	for _, name := range strings.Split(vary, ",") {
		if name = strings.TrimSpace(name); name != "" {
			k += "|" + name + "=" + h.Get(name)
		}
	}
	return k
}

func (e *Edge) Get(url string, h http.Header) string {
	if c, ok := e.store[key(url, e.seenVary[url], h)]; ok {
		e.hits++
		return c.body
	}
	e.misses++
	resp := e.askOrigin(url, h)
	v := resp.Header.Get("Vary")
	e.seenVary[url] = v
	e.store[key(url, v, h)] = copyOf{body(resp), v}
	return e.store[key(url, v, h)].body
}

func (e *Edge) askOrigin(url string, h http.Header) *http.Response {
	req, _ := http.NewRequest("GET", e.origin+url, nil)
	req.Header = h.Clone()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	return resp
}

func body(resp *http.Response) string {
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

// isMobile stands in for a real User-Agent parser.
func isMobile(ua string) bool { return strings.HasSuffix(ua, "0") }

// normalise runs at the edge: two values, not five hundred.
func normalise(r *http.Request) {
	device := "desktop"
	if isMobile(r.Header.Get("User-Agent")) {
		device = "mobile"
	}
	r.Header.Set("X-Device", device)
}

func origin(vary string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if vary != "" {
			w.Header().Set("Vary", vary)
		}
		if r.Header.Get("Accept") == "text/csv" {
			io.WriteString(w, "CSV")
			return
		}
		io.WriteString(w, "JSON")
	}))
}

func accept(v string) http.Header { h := http.Header{}; h.Set("Accept", v); return h }

func main() {
	for _, vary := range []string{"", "Accept"} {
		srv := origin(vary)
		e := &Edge{origin: srv.URL, seenVary: map[string]string{}, store: map[string]copyOf{}}
		fmt.Printf("== Vary: %q ==\n", vary)
		fmt.Printf("  script asks JSON, gets %s\n", e.Get("/facts", accept("application/json")))
		fmt.Printf("  spreadsheet asks CSV, gets %s\n", e.Get("/facts", accept("text/csv")))
		fmt.Printf("  copies at the edge: %d\n", len(e.store))
		srv.Close()
	}

	agents := make([]string, 2000) // every browser version on every device
	for i := range agents {
		agents[i] = fmt.Sprintf("Browser/%d", i%500)
	}
	run := func(name string, value func(string) string) {
		srv := origin("X-Device")
		e := &Edge{origin: srv.URL, seenVary: map[string]string{}, store: map[string]copyOf{}}
		for _, ua := range agents {
			h := http.Header{}
			h.Set("X-Device", value(ua))
			e.Get("/page", h)
		}
		fmt.Printf("  %-26s copies=%3d hit rate=%2.0f%%\n", name, len(e.store), 100*float64(e.hits)/float64(e.hits+e.misses))
		srv.Close()
	}
	fmt.Println("== 2,000 requests from 500 different User-Agent strings ==")
	run("Vary: User-Agent", func(ua string) string { return ua })
	run("normalised at the edge", func(ua string) string {
		r, _ := http.NewRequest("GET", "/page", nil)
		r.Header.Set("User-Agent", ua)
		normalise(r)
		return r.Header.Get("X-Device")
	})
}
