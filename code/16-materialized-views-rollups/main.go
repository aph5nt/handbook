// Materialized views and incremental rollups: daily swap volume.
// Refresh by partition recomputes every day whose facts changed.
// The trap refreshes only rows above a block-height cursor and misses
// a re-indexed old day.
package main

import (
	"fmt"
	"sort"
)

// The StarRocks materialized view the "refresh by partition" tab stands for.
const mvSQL = `-- the slow way: every page load scans every swap
SELECT day, sum(amount_usd) FROM swaps GROUP BY day;

-- a materialized view stores the answer, one partition per day
CREATE MATERIALIZED VIEW daily_volume
PARTITION BY day
REFRESH ASYNC EVERY (INTERVAL 10 MINUTE)
AS SELECT day, sum(amount_usd) AS volume
   FROM   swaps
   GROUP  BY day;

-- the dashboard reads the stored rows
SELECT day, volume FROM daily_volume ORDER BY day;

-- each refresh recomputes only partitions whose base data changed:
-- a new day (5 Oct) or a re-indexed day (2 Oct)
REFRESH MATERIALIZED VIEW daily_volume
  PARTITION START ('2026-10-02') END ('2026-10-03');

-- lane reset: the view is derived, drop and rebuild it with the facts
DROP MATERIALIZED VIEW daily_volume;`

type Swap struct {
	Day    string
	Block  int
	Amount int
}

// Facts holds swaps by day partition, and a version per partition that
// goes up whenever the partition is written.
type Facts struct {
	rows    map[string][]Swap
	version map[string]int
}

func (f *Facts) Write(day string, rows []Swap, replace bool) {
	if replace {
		f.rows[day] = nil
	}
	f.rows[day] = append(f.rows[day], rows...)
	f.version[day]++
}

// PartitionRollup recomputes each day whose version changed since its
// last refresh. Old days are untouched unless their facts changed.
type PartitionRollup struct {
	total   map[string]int
	seenVer map[string]int
}

// sumOf adds up one day of swaps.
func sumOf(rows []Swap) int {
	sum := 0
	for _, s := range rows {
		sum += s.Amount
	}
	return sum
}

func (r *PartitionRollup) Refresh(f *Facts) (recomputed []string) {
	for day, v := range f.version {
		if r.seenVer[day] == v {
			continue // facts unchanged: keep the stored total
		}
		r.total[day] = sumOf(f.rows[day]) // recompute the whole day
		r.seenVer[day] = v
		recomputed = append(recomputed, day)
	}
	sort.Strings(recomputed)
	return recomputed
}

// CursorRollup adds only rows above the highest block it has seen.
type CursorRollup struct {
	total  map[string]int
	cursor int
}

func (r *CursorRollup) Refresh(f *Facts) {
	next := r.cursor
	for _, rows := range f.rows {
		for _, s := range rows {
			if s.Block <= r.cursor {
				continue // assumed already counted: old rows never change
			}
			r.total[s.Day] += s.Amount
			if s.Block > next {
				next = s.Block
			}
		}
	}
	r.cursor = next
}

var days = []string{"1 Oct", "2 Oct", "3 Oct", "4 Oct", "5 Oct"}

func show(label string, t map[string]int) {
	fmt.Printf("%-34s", label)
	for _, d := range days {
		if v, ok := t[d]; ok {
			fmt.Printf(" %s %d |", d, v)
		} else {
			fmt.Printf(" %s empty |", d)
		}
	}
	fmt.Println()
}

func main() {
	f := &Facts{rows: map[string][]Swap{}, version: map[string]int{}}
	f.Write("1 Oct", []Swap{{"1 Oct", 100, 5000}, {"1 Oct", 110, 4100}}, false)
	f.Write("2 Oct", []Swap{{"2 Oct", 200, 4000}, {"2 Oct", 210, 4400}}, false)
	f.Write("3 Oct", []Swap{{"3 Oct", 300, 7950}}, false)
	f.Write("4 Oct", []Swap{{"4 Oct", 400, 8800}}, false)

	pr := &PartitionRollup{total: map[string]int{}, seenVer: map[string]int{}}
	cr := &CursorRollup{total: map[string]int{}}
	pr.Refresh(f)
	cr.Refresh(f)
	show("first build:", pr.total)

	f.Write("5 Oct", []Swap{{"5 Oct", 500, 1500}, {"5 Oct", 510, 800}}, false)
	fmt.Println("refresh at 10:00 recomputes:", pr.Refresh(f))
	cr.Refresh(f)
	show("partition rollup at 10:00:", pr.total)

	// a decoder fix re-indexes 2 October: same blocks, corrected amount
	f.Write("2 Oct", []Swap{{"2 Oct", 200, 4000}, {"2 Oct", 210, 4650}}, true)
	f.Write("5 Oct", []Swap{{"5 Oct", 520, 800}}, false)
	fmt.Println("next refresh recomputes:", pr.Refresh(f))
	cr.Refresh(f)
	show("partition rollup:", pr.total)
	show("cursor rollup (the trap):", cr.total)
	fmt.Printf("facts say 2 Oct = %d, cursor rollup says %d\n", 4000+4650, cr.total["2 Oct"])
	_ = mvSQL
}
