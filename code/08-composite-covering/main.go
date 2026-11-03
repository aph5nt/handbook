// An in-memory model of a composite index: twelve entries sorted by two
// columns, with counters for entries read and table lookups. It needs no
// database, because the leftmost-prefix rule follows from the sort order.
package main

import (
	"fmt"
	"sort"
)

type Entry struct {
	Addr   string
	Height int
}

var rows = []Entry{
	{"0xa1", 20100004}, {"0xa1", 21050300}, {"0xa1", 21700800},
	{"0xb7", 20900010}, {"0xb7", 21400120}, {"0xb7", 21400980}, {"0xb7", 21650000},
	{"0xc3", 19800500}, {"0xc3", 21420000}, {"0xc3", 21990700},
	{"0xd9", 21100000}, {"0xd9", 21600200},
}

// addrFirst is the index (from_address, block_height): seek on both columns.
func addrFirst(addr string, h, limit int, included bool) (read, lookups int, heights []int) {
	idx := append([]Entry{}, rows...)
	sort.Slice(idx, func(i, j int) bool {
		if idx[i].Addr != idx[j].Addr {
			return idx[i].Addr < idx[j].Addr
		}
		return idx[i].Height < idx[j].Height
	})
	pos := sort.Search(len(idx), func(i int) bool {
		e := idx[i]
		return e.Addr > addr || (e.Addr == addr && e.Height > h)
	})
	for ; pos < len(idx) && idx[pos].Addr == addr; pos++ {
		read++
		heights = append(heights, idx[pos].Height)
		if !included {
			lookups++ // to_address and amount live in the table
		}
		if limit > 0 && read == limit {
			break
		}
	}
	return read, lookups, heights
}

// heightFirst is the index (block_height, from_address): only the height seeks.
func heightFirst(addr string, h int) (read, kept int) {
	idx := append([]Entry{}, rows...)
	sort.Slice(idx, func(i, j int) bool { return idx[i].Height < idx[j].Height })
	pos := sort.Search(len(idx), func(i int) bool { return idx[i].Height > h })
	for ; pos < len(idx); pos++ {
		read++
		if idx[pos].Addr == addr {
			kept++
		}
	}
	return read, kept
}

func main() {
	const h = 21400000
	r, l, hs := addrFirst("0xb7", h, 0, false)
	fmt.Printf("(from_address, block_height): read %d, lookups %d, heights %v\n", r, l, hs)
	r, l, _ = addrFirst("0xb7", h, 0, true)
	fmt.Printf("with INCLUDE (to_address, amount): read %d, lookups %d\n", r, l)
	r, _, hs = addrFirst("0xb7", h, 2, true)
	fmt.Printf("page 1, LIMIT 2: read %d, heights %v\n", r, hs)
	r, _, hs = addrFirst("0xb7", hs[len(hs)-1], 2, true)
	fmt.Printf("page 2, seek past the last height: read %d, heights %v\n", r, hs)

	match := 0
	for _, e := range rows {
		if e.Height > h {
			match++
		}
	}
	fmt.Printf("height alone on the same index: read %d, match %d\n", len(rows), match)
	r, k := heightFirst("0xb7", h)
	fmt.Printf("(block_height, from_address): read %d to keep %d\n", r, k)

	matching, total := 9, 12
	fmt.Printf("is_contract = false: index %d + lookups %d = %d reads, table sweep %d reads\n",
		matching, matching, matching*2, total)
}
