// An in-memory model of a plan: generated swaps pass through scan, join and
// filter, with a counter on each. It needs no database. It also proves why a
// window partitioned by another column stops a filter: the ranks change.
package main

import (
	"fmt"
	"sort"
)

type Swap struct {
	Day, Pool, Amount int
}

const (
	daysDrawn   = 7
	rowsPerDay  = 100_000
	columns     = 40
	targetDay   = 4 // 2026-10-01 is the fourth day drawn
	namedColumn = 3
)

func generate() []Swap {
	var rows []Swap
	for d := 1; d <= daysDrawn; d++ {
		for i := 0; i < rowsPerDay; i++ {
			rows = append(rows, Swap{Day: d, Pool: i % 50, Amount: (i*7 + d*13) % 1000})
		}
	}
	return rows
}

func run(rows []Swap, pushed bool) (scanned, joined, kept, parts, cols int) {
	parts, cols = daysDrawn, columns
	if pushed {
		parts, cols = 1, namedColumn
	}
	for _, r := range rows {
		if pushed && r.Day != targetDay {
			continue // pruned at the scan: never read
		}
		scanned++
		joined++ // the join to pools sees this row
		if r.Day == targetDay {
			kept++
		}
	}
	return
}

// rankOf ranks the amounts of one pool, biggest first, and returns the rank of a row.
func rankOf(rows []Swap, pool, amount int) int {
	var a []int
	for _, r := range rows {
		if r.Pool == pool {
			a = append(a, r.Amount)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(a)))
	return sort.Search(len(a), func(i int) bool { return a[i] <= amount }) + 1
}

func main() {
	rows := generate()
	for _, pushed := range []bool{false, true} {
		s, j, k, p, c := run(rows, pushed)
		fmt.Printf("pushed %-5v: partitions %d of %d, columns %d of %d, rows scanned %d, into the join %d, kept %d\n",
			pushed, p, daysDrawn, c, columns, s, j, k)
	}

	small := []Swap{{Day: 1, Pool: 9, Amount: 900}, {Day: 1, Pool: 9, Amount: 800}, {Day: 2, Pool: 9, Amount: 500}, {Day: 2, Pool: 9, Amount: 100}}
	var day2 []Swap
	for _, r := range small {
		if r.Day == 2 {
			day2 = append(day2, r)
		}
	}
	fmt.Printf("rank of the 500 swap, ranked over all days then filtered: %d\n", rankOf(small, 9, 500))
	fmt.Printf("rank of the 500 swap, filtered first, then ranked:         %d\n", rankOf(day2, 9, 500))
}
