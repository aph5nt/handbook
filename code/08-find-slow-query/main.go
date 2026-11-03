// An in-memory model of pg_stat_statements. It needs no database: it writes
// one day of query log lines with real constants, normalises each into a
// statement shape, groups by shape, and ranks the shapes two ways.
package main

import (
	"fmt"
	"regexp"
	"sort"
)

type Shape struct {
	Text           string
	Calls          int
	TotalMs, MaxMs int
}

var literal = regexp.MustCompile(`'[^']*'|\b\d+\b`)

// normalise turns constants into ?, so every page write is one shape.
func normalise(sql string) string { return literal.ReplaceAllString(sql, "?") }

type source struct {
	sqlFmt     string
	perHour    int // calls per hour; 0 means once a day
	durationMs int
}

func day(srcs []source) map[string]*Shape {
	shapes := map[string]*Shape{}
	record := func(sql string, ms int) {
		k := normalise(sql)
		s := shapes[k]
		if s == nil {
			s = &Shape{Text: k}
			shapes[k] = s
		}
		s.Calls++
		s.TotalMs += ms
		if ms > s.MaxMs {
			s.MaxMs = ms
		}
	}
	for _, src := range srcs {
		calls := src.perHour * 24
		if src.perHour == 0 {
			calls = 1
		}
		for i := 0; i < calls; i++ {
			record(fmt.Sprintf(src.sqlFmt, 9_506_000+i), src.durationMs)
		}
	}
	return shapes
}

func show(title string, shapes map[string]*Shape, by func(*Shape) int) []*Shape {
	var list []*Shape
	for _, s := range shapes {
		list = append(list, s)
	}
	sort.Slice(list, func(i, j int) bool { return by(list[i]) > by(list[j]) })
	fmt.Println(title)
	for _, s := range list {
		fmt.Printf("  calls/h %6.0f  max %6d ms  total/h %7.1f s  %s\n",
			float64(s.Calls)/24, s.MaxMs, float64(s.TotalMs)/24/1000, s.Text)
	}
	return list
}

func main() {
	srcs := []source{
		{"INSERT INTO page SELECT ... JOIN addresses a1 ... JOIN addresses a2 ... WHERE block_height = %d", 3000, 2000},
		{"SELECT MAX(block_height) FROM swaps WHERE writer = 'w-%d'", 3000, 300},
		{"SELECT pool, sum(amount) FROM swaps WHERE day = %d GROUP BY pool", 600, 500},
		{"SELECT * FROM transactions WHERE tx_hash = '0x%x'", 50000, 5},
		{"SELECT trader, sum(amount) FROM swaps WHERE day = %d GROUP BY trader ORDER BY 2 DESC LIMIT 100", 0, 60000},
	}
	shapes := day(srcs)
	fmt.Printf("%d shapes from one day of log lines\n", len(shapes))
	show("Ranked by slowest single run (the trap):", shapes, func(s *Shape) int { return s.MaxMs })
	top := show("Ranked by total time = calls x mean:", shapes, func(s *Shape) int { return s.TotalMs })[0]

	all := 0
	for _, s := range shapes {
		all += s.TotalMs
	}
	fmt.Printf("top shape: %.0f of %.1f s per hour, %.0f percent of the load\n",
		float64(top.TotalMs)/24/1000, float64(all)/24/1000, 100*float64(top.TotalMs)/float64(all))
	fmt.Println("After the fix: the page write does one keyed IN lookup, 80 ms")
	srcs[0].durationMs = 80
	fixed := day(srcs)[top.Text]
	fmt.Printf("  top shape total/h: %.0f s -> %.0f s\n", float64(top.TotalMs)/24/1000, float64(fixed.TotalMs)/24/1000)
}
