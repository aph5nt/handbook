// An in-memory model of a table sorted by (block_height, log_index): twenty rows
// and a counter of rows read. It needs no database.
package main

import (
	"fmt"
	"sort"
)

type Key struct{ Height, Log int }

var rows = []Key{
	{101, 0}, {101, 1}, {102, 0}, {103, 0}, {103, 1}, {103, 2}, {104, 0}, {104, 1}, {105, 0}, {106, 0},
	{106, 1}, {107, 0}, {107, 1}, {107, 2}, {108, 0}, {108, 1}, {109, 0}, {110, 0}, {110, 1}, {111, 0},
}

func less(a, b Key) bool {
	if a.Height != b.Height {
		return a.Height < b.Height
	}
	return a.Log < b.Log
}

// offsetPage reads and discards offset rows, then reads limit more.
func offsetPage(offset, limit int) (read int, page []Key) {
	for i, r := range rows {
		read++
		if i >= offset {
			page = append(page, r)
		}
		if len(page) == limit {
			break
		}
	}
	return read, page
}

// keysetPage seeks past the last key, then reads limit rows.
func keysetPage(last Key, limit int) (read int, page []Key) {
	pos := sort.Search(len(rows), func(i int) bool { return less(last, rows[i]) })
	for ; pos < len(rows) && len(page) < limit; pos++ {
		read++
		page = append(page, rows[pos])
	}
	return read, page
}

// heightOnlyPage compares block_height alone, so a row that shares it is skipped.
func heightOnlyPage(last, limit int) (page []Key) {
	for _, r := range rows {
		if r.Height > last && len(page) < limit {
			page = append(page, r)
		}
	}
	return page
}

func main() {
	for _, off := range []int{0, 9, 15} {
		r, _ := offsetPage(off, 3)
		fmt.Printf("OFFSET %2d LIMIT 3: read %2d, returned 3, thrown away %d\n", off, r, off)
	}
	fmt.Printf("page 1,000 at 1,000 rows per page: read %d rows\n", 1000*1000)
	r, p := keysetPage(Key{108, 0}, 3)
	fmt.Printf("keyset after (108,0): read %d, rows %v\n", r, p)
	fmt.Printf("height only, after 108: rows %v, so (108,1) is lost\n", heightOnlyPage(108, 3))

	fmt.Printf("MAX over the table: read %d rows, answer %d\n", len(rows), rows[len(rows)-1].Height)
	fmt.Printf("MAX over the newest day: read 4 rows\n")
	fmt.Printf("ORDER BY key DESC LIMIT 1: read 1 row, answer %d\n", rows[len(rows)-1].Height)

	matches := []int{3, 8, 12, 17} // the rows that went to this address
	fmt.Printf("COUNT(*) > 0: read %d matches\n", len(matches))
	fmt.Printf("SELECT 1 LIMIT 1: stops at row %d\n", matches[0]+1)

	const days, perDay, limit = 700, 1_000_000, 500_000_000
	whole := days * perDay
	fmt.Printf("anti join, whole table: %d hash entries, limit %d: out of memory = %v\n", whole, limit, whole > limit)
	fmt.Printf("anti join, one day: %d hash entries: out of memory = %v\n", perDay, perDay > limit)
	const page = 10_000
	fmt.Printf("anti join, one writer page: at most %d entries\n", page)
}
