// Sixteen swaps, four to a page. Replays the lesson
// 13-pagination-offset-vs-keyset: OFFSET reads every skipped row and repeats
// a row when a new one lands; keyset seeks past the last key and does not.
// The rows are a slice, and rows read counts what a database would touch.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
)

const pageSize = 4

// API holds swaps newest first: the index on block.
type API struct {
	blocks   []int
	rowsRead int
}

func (a *API) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /offset", a.byOffset)
	mux.HandleFunc("GET /keyset", a.byKeyset)
	return mux
}

// byOffset: ORDER BY block DESC LIMIT 4 OFFSET n. It steps over n rows.
func (a *API) byOffset(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	end := min(offset+pageSize, len(a.blocks))
	a.rowsRead += end // every row up to the end of the page is read
	json.NewEncoder(w).Encode(map[string]any{"rows": a.blocks[min(offset, end):end]})
}

// byKeyset: WHERE block < cursor ORDER BY block DESC LIMIT 4. It seeks.
func (a *API) byKeyset(w http.ResponseWriter, r *http.Request) {
	start := 0
	if c := r.URL.Query().Get("cursor"); c != "" {
		raw, err := base64.URLEncoding.DecodeString(c)
		last, err2 := strconv.Atoi(string(raw))
		if err != nil || err2 != nil {
			http.Error(w, "bad cursor", http.StatusBadRequest)
			return
		}
		start = sort.Search(len(a.blocks), func(i int) bool { return a.blocks[i] < last })
	}
	end := min(start+pageSize, len(a.blocks))
	page := a.blocks[start:end]
	a.rowsRead += len(page) // the index jumps: only the page is read
	var next *string
	if end < len(a.blocks) {
		c := base64.URLEncoding.EncodeToString([]byte(strconv.Itoa(page[len(page)-1])))
		next = &c
	}
	json.NewEncoder(w).Encode(map[string]any{"rows": page, "next_cursor": next})
}

type page struct {
	Rows []int   `json:"rows"`
	Next *string `json:"next_cursor"`
}

func (a *API) get(base, path string) page {
	a.rowsRead = 0
	res, err := http.Get(base + path)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	var p page
	json.NewDecoder(res.Body).Decode(&p)
	return p
}

// tieBreak shows a block-only cursor on two swaps in one block.
type swap struct{ block, log int }

func afterBlock(rows []swap, block int, n int) []swap {
	var out []swap
	for _, s := range rows {
		if s.block < block && len(out) < n {
			out = append(out, s)
		}
	}
	return out
}

func afterKey(rows []swap, last swap, n int) []swap {
	var out []swap
	for _, s := range rows {
		if (s.block < last.block || (s.block == last.block && s.log < last.log)) && len(out) < n {
			out = append(out, s)
		}
	}
	return out
}

func main() {
	a := &API{}
	for b := 960; b >= 885; b -= 5 {
		a.blocks = append(a.blocks, b)
	}
	srv := httptest.NewServer(a.routes())
	defer srv.Close()

	fmt.Println("--- offset paging")
	var seen []int
	for _, off := range []int{0, 4, 8} {
		p := a.get(srv.URL, "/offset?offset="+strconv.Itoa(off))
		seen = append(seen, p.Rows...)
		fmt.Println("OFFSET", off, p.Rows, "rows read", a.rowsRead, "returned", len(p.Rows))
	}
	a.blocks = append([]int{965}, a.blocks...) // a new swap lands at the front
	p := a.get(srv.URL, "/offset?offset=12")
	fmt.Println("after block 965 lands, OFFSET 12", p.Rows, "rows read", a.rowsRead)
	fmt.Println("block", p.Rows[0], "already seen on page 3:", contains(seen, p.Rows[0]))

	fmt.Println("--- keyset paging")
	a.blocks = a.blocks[1:]
	k := a.get(srv.URL, "/keyset")
	fmt.Println("page 1", k.Rows, "rows read", a.rowsRead)
	k = a.get(srv.URL, "/keyset?cursor="+*k.Next)
	fmt.Println("page 2", k.Rows, "rows read", a.rowsRead)
	k = a.get(srv.URL, "/keyset?cursor="+*k.Next)
	fmt.Println("page 3", k.Rows, "rows read", a.rowsRead)
	seen = nil
	cursor := *k.Next
	seen = append(seen, k.Rows...)
	a.blocks = append([]int{965}, a.blocks...)
	k = a.get(srv.URL, "/keyset?cursor="+cursor)
	fmt.Println("after block 965 lands, page 4", k.Rows, "rows read", a.rowsRead, "| repeated:", contains(seen, k.Rows[0]))

	fmt.Println("--- the trap: a key that is not unique")
	rows := []swap{{910, 2}, {905, 2}, {905, 1}, {900, 0}}
	fmt.Println("page ended at", rows[1], "| next by block only:", afterBlock(rows, 905, 2), "(905 log 1 is skipped)")
	fmt.Println("next by block and log:", afterKey(rows, rows[1], 2))
}

func contains(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
