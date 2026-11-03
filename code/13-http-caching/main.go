// A CDN in front of a pools API. Replays the lesson 13-http-caching: a copy
// served for 60 seconds, revalidation with 304, a personal answer leaked by
// "public", and the fixes (private, Vary). A fake clock keeps the numbers
// the same on every run.
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
)

var now int // seconds, the fake clock

type entry struct {
	body, etag, varyOn string
	storedAt, maxAge   int
}

// CDN stores public answers and revalidates stale ones.
type CDN struct {
	origin http.Handler
	copies map[string]entry
	hits   int
	calls  int
}

func (c *CDN) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Path
	if e, ok := c.copies[key]; ok && e.varyOn != "" {
		key += "|" + r.Header.Get(e.varyOn) // one copy per value of that header
	}
	e, ok := c.copies[key]
	if ok && now-e.storedAt < e.maxAge {
		c.hits++
		w.Header().Set("ETag", e.etag)
		fmt.Fprint(w, e.body) // fresh: the origin does no work
		return
	}
	if ok {
		r.Header.Set("If-None-Match", e.etag) // stale: ask if it changed
	}
	rec := httptest.NewRecorder()
	c.origin.ServeHTTP(rec, r)
	c.calls++
	res := rec.Result()
	if res.StatusCode == http.StatusNotModified {
		e.storedAt = now
		c.copies[key] = e
		fmt.Fprint(w, e.body)
		return
	}
	cc := res.Header.Get("Cache-Control")
	if strings.Contains(cc, "public") {
		vary := res.Header.Get("Vary")
		if vary != "" {
			c.copies[r.URL.Path] = entry{varyOn: vary, storedAt: -1 << 30}
			key = r.URL.Path + "|" + r.Header.Get(vary)
		}
		c.copies[key] = entry{rec.Body.String(), res.Header.Get("ETag"), vary, now, maxAge(cc)}
	}
	w.Header().Set("Cache-Control", cc)
	fmt.Fprint(w, rec.Body.String())
}

var ageRule = regexp.MustCompile(`max-age=(\d+)`)

func maxAge(cc string) int {
	m := ageRule.FindStringSubmatch(cc)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// API is the origin.
type API struct {
	version    int
	policy     string // Cache-Control of /me/watchlists
	vary       string
	bodiesSent int
}

func (a *API) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /pools/42", func(w http.ResponseWriter, r *http.Request) {
		tag := `"v` + strconv.Itoa(a.version) + `"`
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Header().Set("ETag", tag)
		if r.Header.Get("If-None-Match") == tag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		a.bodiesSent++
		fmt.Fprintf(w, "pool 42 at %s", tag)
	})
	mux.HandleFunc("GET /me/watchlists", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", a.policy)
		if a.vary != "" {
			w.Header().Set("Vary", a.vary)
		}
		fmt.Fprintf(w, "watch list of %s", r.Header.Get("Authorization"))
	})
	mux.HandleFunc("GET /blocks/{n}", func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(r.PathValue("n"))
		if n <= safeHead {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-store")
		}
		fmt.Fprint(w, "block ", n)
	})
	return mux
}

const safeHead = 9_000

func get(url, who string) *http.Response {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", who)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	return res
}

func text(res *http.Response) string {
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return string(body)
}

func main() {
	a := &API{version: 5}
	cdn := &CDN{origin: a.routes(), copies: map[string]entry{}}
	srv := httptest.NewServer(cdn)
	defer srv.Close()

	fmt.Println("--- Cache-Control, ETag and 304")
	fmt.Println("first request:", text(get(srv.URL+"/pools/42", "")), "| origin calls", cdn.calls)
	for i := 0; i < 1000; i++ {
		text(get(srv.URL+"/pools/42", ""))
	}
	fmt.Println("1000 browsers in the minute: CDN hits", cdn.hits, "| origin calls", cdn.calls)
	now = 61
	text(get(srv.URL+"/pools/42", ""))
	fmt.Println("after 60 s, not changed: origin calls", cdn.calls, "| bodies sent", a.bodiesSent)
	now = 122
	a.version = 6
	fmt.Println("after a new swap:", text(get(srv.URL+"/pools/42", "")), "| bodies sent", a.bodiesSent)

	fmt.Println("--- a personal answer cached as public")
	a.policy = "public, max-age=300"
	text(get(srv.URL+"/me/watchlists", "alice"))
	fmt.Println("Bob sees:", text(get(srv.URL+"/me/watchlists", "bob")))
	delete(cdn.copies, "/me/watchlists")
	a.policy = "private"
	text(get(srv.URL+"/me/watchlists", "alice"))
	fmt.Println("private: Bob sees:", text(get(srv.URL+"/me/watchlists", "bob")), "| copies of it", stored(cdn))
	a.policy, a.vary = "public, max-age=300", "Authorization"
	text(get(srv.URL+"/me/watchlists", "alice"))
	fmt.Println("Vary: Bob sees:", text(get(srv.URL+"/me/watchlists", "bob")), "| copies of it", stored(cdn))

	fmt.Println("--- chain data below the safe head")
	fmt.Println("block 8000:", get(srv.URL+"/blocks/8000", "").Header.Get("Cache-Control"))
	fmt.Println("block 9500:", get(srv.URL+"/blocks/9500", "").Header.Get("Cache-Control"))
}

func stored(c *CDN) int {
	n := 0
	for k := range c.copies {
		if strings.HasPrefix(k, "/me/watchlists|") || (k == "/me/watchlists" && c.copies[k].body != "") {
			n++
		}
	}
	return n
}
