// Filters and sorts on a table of swaps. Replays the lesson
// 13-filtering-and-sorting: any field means a full scan per page, an
// allow-list keeps every query on an index. The table is a slice; "rows
// read" stands for what a database would have to touch.
package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
)

type Swap struct {
	Pool, Block, USD, Fee int
}

const tableSize = 100_000

var table = func() []Swap {
	t := make([]Swap, tableSize)
	for i := range t {
		t[i] = Swap{Pool: i % 1000, Block: i, USD: i % 500, Fee: (i * 7) % 100}
	}
	return t
}()

// byPool is the index on pool: its rows, already in block order.
var byPool = func() map[int][]Swap {
	m := map[int][]Swap{}
	for _, s := range table {
		m[s.Pool] = append(m[s.Pool], s)
	}
	return m
}()

var rowsRead int

const scanBudget = 50_000 // a stand-in for the time limit, in rows read

var errTimeout = errors.New("query-timeout")

// run reads through the pool index when it can, else scans the whole table.
func run(pool, minUSD int, sortBy string, limit, budget int) ([]Swap, error) {
	source := table
	if rows, ok := byPool[pool]; ok && pool >= 0 {
		source = rows
	}
	var hits []Swap
	for _, s := range source {
		rowsRead++
		if budget > 0 && rowsRead > budget {
			return nil, errTimeout
		}
		if s.USD >= minUSD && (pool < 0 || s.Pool == pool) {
			hits = append(hits, s)
		}
	}
	if sortBy == "fee" {
		sort.Slice(hits, func(i, j int) bool { return hits[i].Fee < hits[j].Fee })
	} else {
		sort.Slice(hits, func(i, j int) bool { return hits[i].Block > hits[j].Block })
	}
	return hits[:min(limit, len(hits))], nil
}

var allowedSorts = map[string]bool{"block": true, "-block": true}
var allowedFilters = map[string]bool{"pool": true, "day": true, "min_usd": true, "sort": true, "cursor": true}

type state struct {
	Pool, MinUSD int
	Sort         string
}

// handler is the API. allowList false is the "any field allowed" tab.
func handler(allowList bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if allowList {
			for f := range q {
				if !allowedFilters[f] {
					http.Error(w, "unknown filter "+f, http.StatusBadRequest)
					return
				}
			}
			if s := q.Get("sort"); s != "" && !allowedSorts[s] {
				http.Error(w, "allowed sorts: block, -block", http.StatusBadRequest)
				return
			}
		}
		st := state{Pool: atoi(q.Get("pool"), -1), MinUSD: atoi(q.Get("min_usd"), 0), Sort: strings.TrimPrefix(q.Get("sort"), "-")}
		if c := q.Get("cursor"); c != "" { // filters and sort ride in the cursor
			raw, _ := base64.URLEncoding.DecodeString(c)
			json.Unmarshal(raw, &st)
		}
		budget := 0
		if allowList {
			budget = scanBudget
		}
		rows, err := run(st.Pool, st.MinUSD, st.Sort, 100, budget)
		if errors.Is(err, errTimeout) {
			http.Error(w, "query-timeout: narrow the filters", http.StatusServiceUnavailable)
			return
		}
		raw, _ := json.Marshal(st)
		json.NewEncoder(w).Encode(map[string]any{"rows": len(rows), "next_cursor": base64.URLEncoding.EncodeToString(raw)})
	}
}

func atoi(s string, fallback int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return fallback
}

func get(base, query string) (int, string) {
	rowsRead = 0
	res, err := http.Get(base + "/swaps?" + query)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	var body struct {
		Next string `json:"next_cursor"`
	}
	json.NewDecoder(res.Body).Decode(&body)
	return res.StatusCode, body.Next
}

func main() {
	fmt.Println("--- any field allowed, table of", tableSize, "rows")
	anyField := httptest.NewServer(handler(false))
	defer anyField.Close()
	code, _ := get(anyField.URL, "pool=42&sort=-block&min_usd=100")
	fmt.Println("analyst, pool index:", code, "rows read", rowsRead)
	total := 0
	for page := 1; page <= 3; page++ {
		get(anyField.URL, "sort=fee")
		total += rowsRead
		fmt.Println("sort=fee, page", page, "rows read", rowsRead)
	}
	fmt.Println("three pages:", total, "rows read for one caller")

	fmt.Println("--- an allow-list with indexes")
	allow := httptest.NewServer(handler(true))
	defer allow.Close()
	code, next := get(allow.URL, "pool=42&sort=-block&min_usd=100")
	fmt.Println("analyst:", code, "rows read", rowsRead)
	code, _ = get(allow.URL, "cursor="+next)
	fmt.Println("page 2, cursor alone:", code, "rows read", rowsRead)
	code, _ = get(allow.URL, "sort=fee")
	fmt.Println("sort=fee:", code, "rows read", rowsRead)
	code, _ = get(allow.URL, "day=2026-10-24") // here the whole table is one huge day
	fmt.Println("allowed filter, a huge day:", code, "rows read", rowsRead)
}
