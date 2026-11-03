// ORM pitfalls: a tiny fake ORM over a query log. The dashboard shows 100 pools
// and their swaps. The first part shows the hidden costs, the second the fixes
// and the query-count test that catches them.
// Run: go run .
package main

import (
	"fmt"
	"strings"
)

const pools = 100           // pools on the dashboard
const swapsPerPool = 5      // swaps shown per pool
const swapsRows = 1_000_000 // rows in the swaps table

// colBytes are the sizes of the six swaps columns in this model.
var colBytes = map[string]int{"id": 8, "pool": 8, "ts": 8, "amount": 16, "raw_log": 2_000, "decoded": 1_500}

var allCols = []string{"id", "pool", "ts", "amount", "raw_log", "decoded"}

// Log records every statement the ORM sends, as SQL logging would.
type Log struct {
	queries  []string
	bytes    int
	rowsRead int
}

func (l *Log) Run(sql string, rowsRead, rowsSent int, cols []string) {
	l.queries = append(l.queries, sql)
	l.rowsRead += rowsRead
	l.bytes += rowsSent * rowBytes(cols)
}

func rowBytes(cols []string) int {
	n := 0
	for _, c := range cols {
		n += colBytes[c]
	}
	return n
}

// Pool is the object the code sees. Its swaps load on first touch.
type Pool struct {
	ID     int
	log    *Log
	swaps  []int
	loaded bool
}

// Swaps is lazy loading: the first read sends a hidden SELECT *.
func (p *Pool) Swaps() []int {
	if !p.loaded {
		p.log.Run(fmt.Sprintf("SELECT * FROM swaps WHERE pool = %d", p.ID), swapsPerPool, swapsPerPool, allCols)
		p.swaps = make([]int, swapsPerPool)
		p.loaded = true
	}
	return p.swaps
}

// AllPools is session.query(Pool).all(): one query, swaps not loaded yet.
func AllPools(l *Log) []*Pool {
	l.Run("SELECT * FROM pools", pools, pools, []string{"id"})
	ps := make([]*Pool, pools)
	for i := range ps {
		ps[i] = &Pool{ID: i + 1, log: l}
	}
	return ps
}

// PoolsWithSwaps is eager loading with named columns: two queries in all.
func PoolsWithSwaps(l *Log, cols []string) []*Pool {
	ps := AllPools(l)
	l.Run("SELECT "+strings.Join(cols, ", ")+" FROM swaps WHERE pool IN (1, 2, ..., 100)", pools*swapsPerPool, pools*swapsPerPool, cols)
	for _, p := range ps {
		p.swaps = make([]int, swapsPerPool)
		p.loaded = true
	}
	return ps
}

// FindSwaps is WHERE pool = :id. The pool column is text (VARCHAR) holding '42'.
// A number against the text column makes MySQL cast every row's value
// to a number to compare, so the index on pool is skipped.
func FindSwaps(l *Log, id any) {
	switch v := id.(type) {
	case int:
		l.Run(fmt.Sprintf("SELECT ts, amount FROM swaps WHERE pool = %d", v), swapsRows, swapsPerPool, []string{"ts", "amount"})
	case string:
		l.Run("SELECT ts, amount FROM swaps WHERE pool = '"+v+"'", swapsPerPool, swapsPerPool, []string{"ts", "amount"})
	}
}

// Request runs one dashboard request and returns how long the transaction stayed open.
// apiInside is the ORM unit of work: one transaction around everything.
func Request(apiInside bool) (openMs int) {
	const writesMs, apiMs = 10, 2_000
	open := writesMs // BEGIN, the writes, COMMIT
	if apiInside {
		open += apiMs // the price API call runs inside the transaction
	}
	return open
}

// AssertQueries is the query-count test.
func AssertQueries(l *Log, most int) error {
	if len(l.queries) > most {
		return fmt.Errorf("dashboard made %d queries, want at most %d", len(l.queries), most)
	}
	return nil
}

func dashboard(l *Log) {
	for _, p := range AllPools(l) {
		_ = p.Swaps() // looks like a field read
	}
}

func hidden() {
	fmt.Println("== hidden costs")
	l := &Log{}
	ps := AllPools(l)
	fmt.Printf("load 100 pools: queries %d\n", len(l.queries))
	ps[0].Swaps()
	fmt.Printf("first touch of pool.swaps: queries %d, the new one: %s\n", len(l.queries), l.queries[1])
	l = &Log{}
	dashboard(l)
	fmt.Printf("loop over 100 pools: queries %d\n", len(l.queries))
	fmt.Printf("SELECT *: columns %d, shown 2, bytes per row %d, of which shown %d\n", len(allCols), rowBytes(allCols), rowBytes([]string{"ts", "amount"}))
	l = &Log{}
	FindSwaps(l, 42)
	fmt.Printf("pool id 42 as a number against the text column: rows read %d, every value cast, index skipped\n", l.rowsRead)
	fmt.Printf("unit of work around the 2000 ms price API: transaction open %d ms\n", Request(true))
}

func fixed() {
	fmt.Println("== find and fix")
	l := &Log{}
	dashboard(l)
	fmt.Printf("SQL log: %s; %s; ... %d queries\n", l.queries[0], l.queries[1], len(l.queries))
	fmt.Printf("test, at most 3 queries: %v\n", AssertQueries(l, 3))
	l = &Log{}
	PoolsWithSwaps(l, allCols)
	err := AssertQueries(l, 3)
	n := len(l.queries)
	fmt.Printf("eager loading: queries %d, test passes %v\n", n, err == nil)
	cols := []string{"pool", "ts", "amount"}
	fmt.Printf("named columns %v: bytes per row %d instead of %d\n", cols, rowBytes(cols), rowBytes(allCols))
	l = &Log{}
	FindSwaps(l, "42")
	fmt.Printf("pool id '42' as text, the column's own type: rows read %d, index used\n", l.rowsRead)
	fmt.Printf("price API first, then a short transaction: open %d ms\n", Request(false))
}

func main() {
	hidden()
	fixed()
}
