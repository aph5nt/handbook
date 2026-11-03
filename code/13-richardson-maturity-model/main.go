// One pool API at each rung of the Richardson ladder. Replays the lesson
// 13-richardson-maturity-model: what a retry and a cache can decide at
// each level, by reading only the method, the URL and the status.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
)

var reads int // how often the database is read

func pool(id string) (map[string]any, bool) {
	reads++
	if id != "42" {
		return nil, false
	}
	return map[string]any{"id": 42, "fee_bps": 30}, true
}

// level0: one URL, one verb, the action in the body, always 200.
func level0(w http.ResponseWriter, r *http.Request) {
	var cmd struct{ Action, ID string }
	json.NewDecoder(r.Body).Decode(&cmd)
	p, ok := pool(cmd.ID)
	if !ok {
		json.NewEncoder(w).Encode(map[string]string{"error": "no such pool"})
		return // still 200: the error hides in the body
	}
	json.NewEncoder(w).Encode(p)
}

// level1: a URL per resource, but every call is still POST.
func level1(w http.ResponseWriter, r *http.Request) {
	p, _ := pool(r.PathValue("id"))
	json.NewEncoder(w).Encode(p)
}

// level2: GET reads, and a missing pool is a clean 404.
func level2(w http.ResponseWriter, r *http.Request) {
	p, ok := pool(r.PathValue("id"))
	if !ok {
		http.Error(w, "no such pool", http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "max-age=60")
	json.NewEncoder(w).Encode(p)
}

// level3: level 2 plus links to what the caller may do next.
func level3(w http.ResponseWriter, r *http.Request) {
	p, ok := pool(r.PathValue("id"))
	if !ok {
		http.Error(w, "no such pool", http.StatusNotFound)
		return
	}
	links := map[string]string{"swaps": "/pools/42/swaps", "labels": "/pools/42/labels"}
	if r.Header.Get("Authorization") != "" {
		links["watch"] = "/watchlist/items" // only a signed-in caller may watch
	}
	p["_links"] = links
	json.NewEncoder(w).Encode(p)
}

// mayRetry is what generic retry code can decide from the method alone.
func mayRetry(method string) bool { return method == http.MethodGet }

// cached is a client cache: only GET answers with Cache-Control are stored.
type cached struct{ saved map[string]string }

func (c *cached) get(url string) (string, int, bool) {
	if body, ok := c.saved[url]; ok {
		return body, 200, true
	}
	res, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.Header.Get("Cache-Control") != "" {
		c.saved[url] = string(body)
	}
	return string(body), res.StatusCode, false
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api", level0)
	mux.HandleFunc("POST /l1/pools/{id}", level1)
	mux.HandleFunc("GET /l2/pools/{id}", level2)
	mux.HandleFunc("GET /l3/pools/{id}", level3)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	fmt.Println("--- level 0")
	res, err := http.Post(srv.URL+"/api", "application/json", strings.NewReader(`{"action":"getPool","id":"99"}`))
	if err != nil {
		panic(err)
	}
	res.Body.Close()
	fmt.Println("missing pool 99:", res.StatusCode, "| may retry POST:", mayRetry("POST"))

	fmt.Println("--- level 1")
	res, err = http.Post(srv.URL+"/l1/pools/42", "", nil)
	if err != nil {
		panic(err)
	}
	res.Body.Close()
	fmt.Println("POST /pools/42:", res.StatusCode, "| may retry POST:", mayRetry("POST"))

	fmt.Println("--- level 2")
	c := &cached{saved: map[string]string{}}
	reads = 0
	_, code, _ := c.get(srv.URL + "/l2/pools/42")
	fmt.Println("GET /pools/42:", code, "| may retry GET:", mayRetry("GET"), "| database reads", reads)
	_, _, hit := c.get(srv.URL + "/l2/pools/42")
	fmt.Println("second read: from cache", hit, "| database reads", reads)
	_, code, _ = c.get(srv.URL + "/l2/pools/99")
	fmt.Println("GET /pools/99:", code)

	fmt.Println("--- level 3")
	for _, token := range []string{"", "t1"} {
		req, _ := http.NewRequest("GET", srv.URL+"/l3/pools/42", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			panic(err)
		}
		var body struct {
			Links map[string]string `json:"_links"`
		}
		json.NewDecoder(res.Body).Decode(&body)
		res.Body.Close()
		fmt.Println("signed in", token != "", "links", len(body.Links))
	}
}
