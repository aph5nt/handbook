// An agent asks a graph API for the links of an address. Replays the lesson
// 13-status-codes-problem-details: 200 with an error inside, then the status
// ladder with RFC 7807 problem details.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
)

type Problem struct {
	Type          string   `json:"type"`
	Title         string   `json:"title"`
	Status        int      `json:"status"`
	Detail        string   `json:"detail,omitempty"`
	InvalidParams []string `json:"invalid_params,omitempty"`
	RequestID     string   `json:"request_id,omitempty"`
}

var hexAddr = regexp.MustCompile(`^0x[0-9a-f]+$`)

func problem(w http.ResponseWriter, p Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	if p.Status == http.StatusTooManyRequests {
		w.Header().Set("Retry-After", "20")
	}
	w.WriteHeader(p.Status)
	json.NewEncoder(w).Encode(p)
}

type API struct {
	calls map[string]int // per token, this minute
	links map[string]int // address -> number of links
}

// links walks the ladder: the first "no" picks the status code.
func (a *API) linksHandler(w http.ResponseWriter, r *http.Request) {
	addr := r.PathValue("addr")
	if !hexAddr.MatchString(addr) { // well-formed?
		problem(w, Problem{Type: "/problems/invalid-params", Title: "Invalid request", Status: 400, InvalidParams: []string{"addr: not hex"}})
		return
	}
	tok := r.Header.Get("Authorization")
	if tok == "" { // who are you?
		problem(w, Problem{Type: "/problems/no-token", Title: "Unauthorized", Status: 401})
		return
	}
	if a.calls[tok]++; a.calls[tok] > 5 { // over budget?
		problem(w, Problem{Type: "/problems/rate-limit", Title: "Too many requests", Status: 429})
		return
	}
	if addr == "0xcd" { // a server bug: say so safely
		problem(w, Problem{Type: "/problems/internal", Title: "Internal error", Status: 500, RequestID: "r-81"})
		return
	}
	n, ok := a.links[addr]
	if !ok || n == 0 {
		problem(w, Problem{Type: "/problems/anchor-missing", Title: "No links yet", Status: 404, Detail: addr + " has no links yet"})
		return
	}
	fmt.Fprintf(w, `{"links":%d}`, n)
}

var msg = "no link yet" // the text changes in a later release

// badHandler answers 200 for everything and hides the error in the body.
func badHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `{"error":%q}`, msg)
}

// Monitor counts status codes, the way a dashboard does.
type Monitor struct{ byCode map[int]int }

func (m *Monitor) wrap(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		m.byCode[rec.Code]++
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		w.Write(rec.Body.Bytes())
	})
}

func (m *Monitor) errorRate() string {
	total, bad := 0, 0
	for c, n := range m.byCode {
		total += n
		if c >= 400 {
			bad += n
		}
	}
	return fmt.Sprintf("%d of %d errors", bad, total)
}

func get(url, tok string) (int, string, http.Header) {
	req, _ := http.NewRequest("GET", url, nil)
	if tok != "" {
		req.Header.Set("Authorization", tok)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, strings.TrimSpace(string(b)), res.Header
}

func main() {
	fmt.Println("--- 200 with an error inside")
	mon := &Monitor{byCode: map[int]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /addresses/{addr}/links", badHandler)
	srv := httptest.NewServer(mon.wrap(mux))
	code, body, _ := get(srv.URL+"/addresses/0xab/links", "t1")
	fmt.Println("GET 0xab ->", code, body, "| cacheable:", code == 200, "| board:", mon.errorRate())
	msg = "no links yet" // a release fixes the typo
	code, body, _ = get(srv.URL+"/addresses/0xab/links", "t1")
	matches := strings.Contains(body, "no link yet") // the agent matches on the old words
	fmt.Println("after the release ->", code, body, "| agent match:", matches, "-> it retries | board:", mon.errorRate())
	srv.Close()

	fmt.Println("--- status code plus problem details")
	mon = &Monitor{byCode: map[int]int{}}
	api := &API{calls: map[string]int{}, links: map[string]int{"0xab": 0}}
	mux = http.NewServeMux()
	mux.HandleFunc("GET /addresses/{addr}/links", api.linksHandler)
	srv = httptest.NewServer(mon.wrap(mux))
	code, body, _ = get(srv.URL+"/addresses/0xzz/links", "t1")
	fmt.Println("GET 0xzz ->", code, body)
	code, body, _ = get(srv.URL+"/addresses/0xab/links", "")
	fmt.Println("GET no token ->", code, body)
	code, body, _ = get(srv.URL+"/addresses/0xab/links", "t1")
	var p Problem
	json.Unmarshal([]byte(body), &p)
	fmt.Println("GET 0xab ->", code, "type", p.Type, "| cacheable:", code == 200)
	fmt.Println("agent matches type == /problems/anchor-missing:", p.Type == "/problems/anchor-missing", "-> asks the user")
	for i := 0; i < 4; i++ { // calls 2 to 5 this minute
		get(srv.URL+"/addresses/0xab/links", "t1")
	}
	code, _, h := get(srv.URL+"/addresses/0xab/links", "t1")
	fmt.Println("6th call this minute ->", code, "Retry-After", h.Get("Retry-After"))
	api.calls = map[string]int{} // a new minute
	code, body, _ = get(srv.URL+"/addresses/0xcd/links", "t1")
	fmt.Println("GET 0xcd ->", code, body)
	fmt.Println("board:", mon.errorRate())
	srv.Close()
}
