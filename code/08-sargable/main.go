// An in-memory model of an index: ten sorted entries and a counter of how many
// entries each predicate reads. It needs no database, because the rule it
// proves is about sorted order, not about any one engine.
package main

import (
	"fmt"
	"sort"
	"strings"
)

// seek is a binary search for lo, then a read until an entry reaches hi.
// hi == "" means no upper end.
func seek(index []string, lo, hi string, strict bool) (read, match int) {
	pos := sort.Search(len(index), func(i int) bool {
		if strict {
			return index[i] > lo
		}
		return index[i] >= lo
	})
	for ; pos < len(index); pos++ {
		if hi != "" && index[pos] >= hi {
			break // past the end of the range: stop
		}
		read++
		match++
	}
	return read, match
}

// scan computes the left side for every entry, because the column is not bare.
func scan(index []string, keep func(string) bool) (read, match int) {
	for _, e := range index {
		read++
		if keep(e) {
			match++
		}
	}
	return read, match
}

// addOneHour is block_ts + interval '1 hour' for the entries of this story.
func addOneHour(e string) string {
	var mo, d, h, m int
	fmt.Sscanf(e, "%d-%d %d:%d", &mo, &d, &h, &m)
	h++
	if h == 24 {
		h, d = 0, d+1
	}
	return fmt.Sprintf("%02d-%02d %02d:%02d", mo, d, h, m)
}

func show(label string, read, match int) {
	fmt.Printf("  %-46s read %2d, match %d\n", label, read, match)
}

func main() {
	ts := []string{"09-30 22:10", "09-30 23:55", "10-01 00:05", "10-01 09:40", "10-01 23:20",
		"10-02 00:30", "10-02 11:15", "10-02 18:00", "10-03 02:45", "10-03 13:10"}
	names := []string{"aero-v2", "aerodrome", "balancer", "curve", "pancake",
		"raphael", "topaz", "uni-v3", "uniswap", "velodrome"}

	fmt.Println("Timestamps")
	r, m := scan(ts, func(e string) bool { return strings.HasPrefix(e, "10-01") })
	show("date(block_ts) = '10-01'", r, m)
	r, m = seek(ts, "10-01 00:00", "10-02 00:00", false)
	show("block_ts >= '10-01' AND < '10-02'", r, m)
	t := "10-02 12:00"
	r, m = scan(ts, func(e string) bool { return addOneHour(e) > t })
	show("block_ts + 1 hour > 10-02 12:00", r, m)
	r, m = seek(ts, "10-02 11:00", "", true)
	show("block_ts > 10-02 11:00", r, m)

	fmt.Println("Text")
	r, m = scan(names, func(e string) bool { return strings.HasSuffix(e, "drome") })
	show("name LIKE '%drome'", r, m)
	r, m = seek(names, "aero", "aerp", false)
	show("name LIKE 'aero%'", r, m)
	r, m = scan(names, func(e string) bool { return e == "curve" })
	show("CONVERT_IMPLICIT(name) = N'curve'", r, m)
	r, m = seek(names, "curve", "curve\x00", false)
	show("name = 'curve'", r, m)
}
