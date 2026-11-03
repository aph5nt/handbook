// An API gateway in front of the MCP proxy pods. Replays the lesson
// 13-gateway-rate-limits: token check, 5 calls per minute per caller,
// RateLimit headers, 429 with Retry-After, a bare 429 storm, and a
// business rule hidden in the gateway.
package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
)

const limit = 5 // calls per caller per 60-second window

// Gateway checks the token, counts calls and forwards to the pods.
type Gateway struct {
	next       http.Handler
	now        int // seconds, a fake clock
	used       map[string]int
	retryAfter bool           // false: the bad tab, a bare 429
	codes      map[string]int // caller+status -> count, for the board
	podCalls   int
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	caller, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || caller == "" {
		g.count("anonymous", 401)
		http.Error(w, "no token", http.StatusUnauthorized)
		return
	}
	window := g.now / 60
	key := caller + "@" + strconv.Itoa(window)
	reset := (window+1)*60 - g.now
	if g.used[key] >= limit {
		if g.retryAfter {
			w.Header().Set("Retry-After", strconv.Itoa(reset))
		}
		g.count(caller, 429)
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return // the pods never see this call
	}
	g.used[key]++
	w.Header().Set("RateLimit-Limit", strconv.Itoa(limit))
	w.Header().Set("RateLimit-Remaining", strconv.Itoa(limit-g.used[key]))
	w.Header().Set("RateLimit-Reset", strconv.Itoa(reset))
	g.count(caller, 200)
	g.podCalls++
	g.next.ServeHTTP(w, r)
}

func (g *Gateway) count(caller string, code int) { g.codes[caller+" "+strconv.Itoa(code)]++ }

func newGateway(next http.Handler, retryAfter bool) *Gateway {
	return &Gateway{next: next, used: map[string]int{}, codes: map[string]int{}, retryAfter: retryAfter}
}

func call(url, token string) *http.Response {
	req, _ := http.NewRequest("POST", url+"/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	res.Body.Close()
	return res
}

// pods is the proxy behind the gateway. labelField is what it calls labels.
func pods(labelField string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"address": "0xab", labelField: []string{"Exchange"}})
	})
}

// freeFilter is the business rule someone put in the gateway: hide labels.
func freeFilter(body map[string]any) { delete(body, "labels") }

func main() {
	fmt.Println("--- limits with Retry-After")
	g := newGateway(pods("labels"), true)
	srv := httptest.NewServer(g)
	g.now = 20
	call(srv.URL, "agent")
	res := call(srv.URL, "agent")
	fmt.Println("agent call 2: Limit", res.Header.Get("RateLimit-Limit"), "Remaining", res.Header.Get("RateLimit-Remaining"), "Reset", res.Header.Get("RateLimit-Reset"))
	g.now = 40
	for i := 1; i <= 5; i++ {
		res = call(srv.URL, "script")
	}
	fmt.Println("script calls 1-5:", res.StatusCode, "Remaining", res.Header.Get("RateLimit-Remaining"))
	res = call(srv.URL, "script")
	fmt.Println("script call 6:", res.StatusCode, "Retry-After", res.Header.Get("Retry-After"), "pod calls", g.podCalls)
	wait, _ := strconv.Atoi(res.Header.Get("Retry-After"))
	g.now += wait
	res = call(srv.URL, "script")
	fmt.Println("after", wait, "s:", res.StatusCode)
	call(srv.URL, "")
	fmt.Println("board:", g.codes)
	srv.Close()

	fmt.Println("--- 429 with no Retry-After")
	rng := rand.New(rand.NewSource(7))
	bare, told := map[int]int{}, map[int]int{}
	for c := 0; c < 300; c++ {
		bare[0]++               // no hint: retry at once
		told[20+rng.Intn(20)]++ // Retry-After 20, plus a spread
	}
	fmt.Println("bare 429: retries in the busiest second:", busiest(bare))
	fmt.Println("Retry-After plus spread: retries in the busiest second:", busiest(told))

	fmt.Println("--- business rule in the gateway")
	for _, field := range []string{"labels", "label_names"} {
		var body map[string]any
		rec := httptest.NewRecorder()
		pods(field).ServeHTTP(rec, httptest.NewRequest("GET", "/mcp", nil))
		json.Unmarshal(rec.Body.Bytes(), &body)
		freeFilter(body)
		_, leak := body[field]
		fmt.Printf("pods call it %q: free user sees labels %v\n", field, leak)
	}
}

func busiest(perSecond map[int]int) int {
	top := 0
	for _, n := range perSecond {
		top = max(top, n)
	}
	return top
}
