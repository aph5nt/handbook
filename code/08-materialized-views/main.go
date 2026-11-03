// An in-memory model of a materialized view: a base table of generated swaps, a
// stored aggregate, a rewriter that matches group keys, and a staleness check.
// It needs no database. It counts the rows each answer reads.
package main

import "fmt"

type Swap struct {
	Day, Pool, Amount, Sender int
}

type Key struct{ Day, Pool int }

type View struct {
	Sum      map[Key]int
	Senders  map[Key]map[int]bool // a mergeable set, standing in for a bitmap or HLL
	Distinct map[Key]int          // a plain distinct count, which cannot be summed
	Through  int                  // the last day refreshed
}

func refreshDay(v *View, base []Swap, day int) {
	for _, s := range base {
		if s.Day != day {
			continue
		}
		k := Key{s.Day, s.Pool}
		v.Sum[k] += s.Amount
		if v.Senders[k] == nil {
			v.Senders[k] = map[int]bool{}
		}
		if !v.Senders[k][s.Sender] {
			v.Senders[k][s.Sender] = true
			v.Distinct[k]++
		}
	}
	v.Through = day
}

const (
	days, pools = 7, 30
	rowsPerDay  = 100_000
	staleLimit  = 0 // days the view may be behind and still answer
)

func main() {
	var base []Swap
	for d := 1; d <= days; d++ {
		for i := 0; i < rowsPerDay; i++ {
			base = append(base, Swap{Day: d, Pool: i % pools, Amount: 1, Sender: i % 1000})
		}
	}
	v := &View{Sum: map[Key]int{}, Senders: map[Key]map[int]bool{}, Distinct: map[Key]int{}}
	for d := 1; d <= days; d++ {
		refreshDay(v, base, d)
	}
	fmt.Printf("no view: %d rows read per load\n", len(base))
	fmt.Printf("view:    %d rows read per load\n", len(v.Sum))

	total := 0
	read := 0
	for k, s := range v.Sum {
		if k.Day == 7 {
			total += s
			read++
		}
	}
	fmt.Printf("roll up, total for day 7: %d, from %d view rows\n", total, read)

	// A new day lands. Only that day is refreshed.
	for i := 0; i < rowsPerDay; i++ {
		base = append(base, Swap{Day: 8, Pool: i % pools, Amount: 1, Sender: i % 1000})
	}
	today := 8
	behind := today - v.Through
	fmt.Printf("day 8 landed, view through day %d: %d day behind\n", v.Through, behind)
	stale := 0
	for k, s := range v.Sum {
		if k.Day == today {
			stale += s
		}
	}
	fmt.Printf("  no limit: day 8 from the view = %d, the truth is %d\n", stale, rowsPerDay)
	if behind > staleLimit {
		fresh := 0
		for _, s := range base {
			if s.Day == today {
				fresh += s.Amount
			}
		}
		fmt.Printf("  limit %d: too stale, read the base table for day 8 = %d\n", staleLimit, fresh)
	}
	refreshDay(v, base, today)
	fmt.Printf("refresh of day 8 reads %d rows of the %d in the table\n", rowsPerDay, len(base))

	sum, merged := 0, map[int]bool{}
	for k, n := range v.Distinct {
		if k.Day == 7 {
			sum += n
			for s := range v.Senders[k] {
				merged[s] = true
			}
		}
	}
	fmt.Printf("distinct senders on day 7: sum of per-pool counts %d, merged sets %d\n", sum, len(merged))
}
