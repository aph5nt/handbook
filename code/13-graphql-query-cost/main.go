// A GraphQL query priced before it runs. Replays the lesson
// 13-graphql-query-cost. A query is a small Go value, not a parsed string,
// because the program uses the standard library only.
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
)

// Field is one selection: its list size and what is selected under it.
type Field struct {
	Name  string  `json:"name"`
	First int     `json:"first,omitempty"`
	Sub   []Field `json:"sub,omitempty"`
}

const (
	maxDepth = 5
	maxFirst = 100
	budget   = 10_000
)

// schema says which fields each type has, and the loop is legal.
var schema = map[string]bool{"address": true, "transfers": true, "sender": true, "hash": true}

// nested builds transfers of senders of transfers: ten levels with the root.
func nested(first int) Field {
	f := Field{Name: "transfers", First: first}
	for i := 0; i < 4; i++ {
		f = Field{Name: "transfers", First: first, Sub: []Field{{Name: "sender", Sub: []Field{f}}}}
	}
	return Field{Name: "address", Sub: []Field{f}}
}

// validate checks the query against the schema and the size rule.
func validate(f Field) error {
	if !schema[f.Name] {
		return fmt.Errorf("unknown field %s", f.Name)
	}
	if f.First > maxFirst {
		return fmt.Errorf("first: %d is over the maximum %d", f.First, maxFirst)
	}
	for _, s := range f.Sub {
		if err := validate(s); err != nil {
			return err
		}
	}
	return nil
}

// price walks each path: list sizes multiply, every node adds its rows.
func price(f Field, rows float64) (depth int, cost float64) {
	if f.First > 0 {
		rows *= float64(f.First)
	}
	depth, cost = 1, rows
	for _, s := range f.Sub {
		d, c := price(s, rows)
		depth = max(depth, d+1)
		cost += c
	}
	return depth, cost
}

// Server admits a query or refuses it with the limit it broke.
type Server struct {
	limits   bool
	resolved float64
	known    map[string]Field // persisted queries, by id
	public   bool
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID    string `json:"id"`
		Query *Field `json:"query"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if s.public {
		q, ok := s.known[req.ID]
		if !ok {
			http.Error(w, "unknown query: only registered ids", http.StatusBadRequest)
			return
		}
		req.Query = &q
	}
	if s.limits {
		if err := validate(*req.Query); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		depth, cost := price(*req.Query, 1)
		if depth > maxDepth {
			http.Error(w, fmt.Sprintf("depth %d, maximum %d", depth, maxDepth), http.StatusBadRequest)
			return
		}
		if cost > budget {
			http.Error(w, fmt.Sprintf("cost %.0f, budget %d", cost, budget), http.StatusBadRequest)
			return
		}
	}
	_, cost := price(*req.Query, 1)
	s.resolved += cost // the resolvers run: this is the database's work
	fmt.Fprintf(w, "ran, rows %.0f", cost)
}

func send(url string, body any) string {
	raw, _ := json.Marshal(body)
	res, err := http.Post(url, "application/json", strings.NewReader(string(raw)))
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	buf := make([]byte, 200)
	n, _ := res.Body.Read(buf)
	return fmt.Sprintf("%d %s", res.StatusCode, strings.TrimSpace(string(buf[:n])))
}

func main() {
	deep := nested(100)
	depth, cost := price(deep, 1)
	fmt.Println("the ten-level query: depth", depth, "| rows in theory", fmt.Sprintf("%.3g", cost))
	fmt.Printf("100 to the power of 5: %.0f (5 of the 10 levels are lists)\n", math.Pow(100, 5))

	fmt.Println("--- no cost limit")
	open := &Server{}
	a := httptest.NewServer(open)
	defer a.Close()
	fmt.Println("no limit:", send(a.URL, map[string]any{"query": deep}), "| database work", fmt.Sprintf("%.3g", open.resolved))
	fmt.Println("a 30 s timeout stops it after the work is done, and it can be sent again")

	fmt.Println("--- priced before it runs")
	priced := &Server{limits: true}
	b := httptest.NewServer(priced)
	defer b.Close()
	big := Field{Name: "transfers", First: 5000, Sub: []Field{{Name: "hash"}}}
	fmt.Println("first 5000:", send(b.URL, map[string]any{"query": big}))
	fmt.Println("the ten-level query:", send(b.URL, map[string]any{"query": deep}), "| database work", priced.resolved)
	wide := Field{Name: "address", Sub: []Field{{Name: "transfers", First: 100, Sub: []Field{{Name: "sender", Sub: []Field{{Name: "transfers", First: 100, Sub: []Field{{Name: "sender"}}}}}}}}}
	fmt.Println("depth is fine, the cost is not:", send(b.URL, map[string]any{"query": wide}))
	cheap := Field{Name: "transfers", First: 20, Sub: []Field{{Name: "hash"}}}
	fmt.Println("two levels, first 20:", send(b.URL, map[string]any{"query": cheap}))

	pub := &Server{limits: true, public: true, known: map[string]Field{"top-transfers-v1": cheap}}
	c := httptest.NewServer(pub)
	defer c.Close()
	fmt.Println("public, registered id:", send(c.URL, map[string]any{"id": "top-transfers-v1"}))
	fmt.Println("public, any other text:", send(c.URL, map[string]any{"query": cheap}))
}
