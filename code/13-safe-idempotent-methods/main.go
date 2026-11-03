// A watch list API. Replays the lesson 13-safe-idempotent-methods: send
// each call twice, as a retry after a lost answer does, and compare the
// state after one call with the state after two.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

// List is one watch list.
type List struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
}

// API stores the lists and the labels.
type API struct {
	lists  map[string]*List
	labels map[string]bool
	nextID int
	keys   map[string]string // POST idempotency key -> created id
}

func (a *API) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /watchlists/{id}", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(a.lists[r.PathValue("id")]) // reads, changes nothing
	})
	mux.HandleFunc("PUT /watchlists/{id}", func(w http.ResponseWriter, r *http.Request) {
		var l List
		json.NewDecoder(r.Body).Decode(&l)
		a.lists[r.PathValue("id")] = &l // replaces everything it holds
	})
	mux.HandleFunc("PATCH /watchlists/{id}", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Name   *string `json:"name"`
			Append string  `json:"append"`
		}
		json.NewDecoder(r.Body).Decode(&p)
		l := a.lists[r.PathValue("id")]
		if p.Name != nil {
			l.Name = *p.Name // sets a value: repeating changes nothing
		}
		if p.Append != "" {
			l.Addresses = append(l.Addresses, p.Append) // relative: repeating adds again
		}
	})
	mux.HandleFunc("POST /watchlists", func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if id, ok := a.keys[key]; ok && key != "" {
			fmt.Fprint(w, id) // same key: same list, nothing new
			return
		}
		a.nextID++
		id := fmt.Sprint(a.nextID)
		a.lists[id] = &List{Name: "new"}
		a.keys[key] = id
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, id)
	})
	mux.HandleFunc("DELETE /labels/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !a.labels[r.PathValue("id")] {
			w.WriteHeader(http.StatusNotFound) // nothing left to delete
			return
		}
		delete(a.labels, r.PathValue("id"))
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func main() {
	a := &API{lists: map[string]*List{"7": {Name: "Old", Addresses: []string{"A"}}},
		labels: map[string]bool{"3": true}, nextID: 7, keys: map[string]string{}}
	srv := httptest.NewServer(a.routes())
	defer srv.Close()
	send := func(method, path, body, key string) int {
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			panic(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	state := func() string { b, _ := json.Marshal(a.lists["7"]); return string(b) }

	fmt.Println("--- methods safe to repeat")
	send("GET", "/watchlists/7", "", "")
	send("GET", "/watchlists/7", "", "")
	fmt.Println("GET twice:", state())
	whole := `{"name":"Old","addresses":["A","B","C"]}`
	send("PUT", "/watchlists/7", whole, "")
	first := state()
	send("PUT", "/watchlists/7", whole, "")
	fmt.Println("PUT twice, same state:", first == state(), state())
	fmt.Println("DELETE label 3:", send("DELETE", "/labels/3", "", ""), "then", send("DELETE", "/labels/3", "", ""), "| labels left", len(a.labels))
	send("PATCH", "/watchlists/7", `{"name":"Exchanges"}`, "")
	send("PATCH", "/watchlists/7", `{"name":"Exchanges"}`, "")
	fmt.Println("PATCH name twice:", state())

	fmt.Println("--- methods that are not")
	send("PATCH", "/watchlists/7", `{"append":"D"}`, "")
	send("PATCH", "/watchlists/7", `{"append":"D"}`, "")
	fmt.Println("PATCH append D twice:", state())
	send("POST", "/watchlists", "", "")
	send("POST", "/watchlists", "", "")
	fmt.Println("POST twice, no key: lists", len(a.lists))
	send("PUT", "/watchlists/7", `{"name":"Renamed"}`, "")
	fmt.Println("PUT with only a name:", state())

	fmt.Println("--- the fix")
	before := len(a.lists)
	send("POST", "/watchlists", "", "k1")
	send("POST", "/watchlists", "", "k1")
	fmt.Println("POST twice, same key: new lists", len(a.lists)-before)
}
