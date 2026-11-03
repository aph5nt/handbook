// Labels for 500 addresses. Replays the lesson 13-batch-endpoints: one call
// per address against a rate limit, one batch call with a result per item,
// the all-or-nothing failure, the missing size limit, and a batch write key.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
)

const maxBatch = 1000
const perMinute = 5

var queries int

func valid(a string) bool { return len(a) == 42 && a[:2] == "0x" }

func addresses() []string {
	out := make([]string, 500)
	for i := range out {
		out[i] = fmt.Sprintf("0x%040d", i)
	}
	for _, bad := range []int{7, 120, 333} {
		out[bad] = "not-an-address-" + strconv.Itoa(bad)
	}
	return out
}

// Result is one answer, in the same place as its input.
type Result struct {
	Address string   `json:"address"`
	Labels  []string `json:"labels,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// batchGet answers per item: bad items never spoil the good ones.
func batchGet(w http.ResponseWriter, r *http.Request) {
	var req struct{ Addresses []string }
	json.NewDecoder(r.Body).Decode(&req)
	if len(req.Addresses) > maxBatch {
		http.Error(w, fmt.Sprintf("at most %d per call", maxBatch), http.StatusRequestEntityTooLarge)
		return
	}
	queries++ // one query for all the valid addresses
	out := make([]Result, len(req.Addresses))
	for i, a := range req.Addresses {
		if valid(a) {
			out[i] = Result{Address: a, Labels: []string{"Exchange"}}
		} else {
			out[i] = Result{Address: a, Error: "invalid address"}
		}
	}
	json.NewEncoder(w).Encode(out)
}

// allOrNothing is the other batch API: one bad item fails the whole call.
func allOrNothing(w http.ResponseWriter, r *http.Request) {
	var req struct{ Addresses []string }
	json.NewDecoder(r.Body).Decode(&req)
	for _, a := range req.Addresses {
		if !valid(a) {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
	}
	json.NewEncoder(w).Encode(req.Addresses)
}

// oneByOne is the first tab: a query per address behind a rate limit.
func oneByOne(as []string) (calls, bad, minutes int) {
	used := 0
	for _, a := range as {
		if used == perMinute {
			minutes++
			used = 0
		}
		used++
		calls++
		queries++
		if !valid(a) {
			bad++
		}
	}
	return calls, bad, minutes + 1
}

// Writes applies a batch write once per idempotency key.
type Writes struct {
	seen    map[string]bool
	applied int
}

func (b *Writes) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	var req struct{ Labels []string }
	json.NewDecoder(r.Body).Decode(&req)
	if !b.seen[key] {
		b.seen[key] = true
		b.applied += len(req.Labels)
	}
	json.NewEncoder(w).Encode(map[string]any{"atomic": true, "applied": b.applied})
}

func send(h http.Handler, as []string) *http.Response {
	srv := httptest.NewServer(h)
	defer srv.Close()
	body, _ := json.Marshal(map[string][]string{"Addresses": as})
	res, err := http.Post(srv.URL+"/labels:batchGet", "application/json", bytes.NewReader(body))
	if err != nil {
		panic(err)
	}
	return res
}

func main() {
	as := addresses()
	fmt.Println("--- one call per address")
	calls, bad, minutes := oneByOne(as)
	fmt.Println("calls", calls, "| each bad address its own 400:", bad, "| minutes at 5 per minute", minutes, "| queries", queries)

	fmt.Println("--- one batch call")
	queries = 0
	res := send(http.HandlerFunc(batchGet), as)
	var out []Result
	json.NewDecoder(res.Body).Decode(&out)
	res.Body.Close()
	good, failed := 0, 0
	for _, r := range out {
		if r.Error == "" {
			good++
		} else {
			failed++
		}
	}
	fmt.Println("status", res.StatusCode, "| results", len(out), "| labels", good, "| errors", failed, "| queries", queries)
	fmt.Println("item 7:", out[7].Address, out[7].Error)

	res = send(http.HandlerFunc(allOrNothing), as)
	res.Body.Close()
	fmt.Println("the other batch API: status", res.StatusCode, "| good results returned 0")

	res = send(http.HandlerFunc(batchGet), make([]string, 2_000_000))
	res.Body.Close()
	fmt.Println("2,000,000 addresses:", res.StatusCode)

	b := &Writes{seen: map[string]bool{}}
	srv := httptest.NewServer(b)
	defer srv.Close()
	for try := 1; try <= 2; try++ {
		body, _ := json.Marshal(map[string][]string{"Labels": make([]string, 500)})
		req, _ := http.NewRequest("POST", srv.URL, bytes.NewReader(body))
		req.Header.Set("Idempotency-Key", "w1")
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			panic(err)
		}
		r.Body.Close()
	}
	fmt.Println("batch write sent twice, same key: applied", b.applied)
}
