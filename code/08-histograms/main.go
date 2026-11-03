// An in-memory model of selectivity estimates. It needs no database: it
// builds the swaps-per-pool counts of the lesson, collects the statistics a
// planner keeps (row count, distinct count, most common values), and prints
// the estimate and the plan chosen for each pool.
package main

import (
	"fmt"
	"sort"
)

const (
	totalRows    = 10_000_000
	distinct     = 20_000
	mcvSize      = 5      // how many most common values the histogram keeps
	loopMaxRows  = 10_000 // the plan rule: nested loop up to this many rows
	tinyPoolRows = 200
)

type Stats struct {
	Rows, Distinct int64
	MCV            map[string]int64 // value -> real row count
	mcvRows        int64
}

// estimate is the planner's guess for WHERE pool = value.
func (s Stats) estimate(value string, known bool) int64 {
	if !known || s.MCV == nil {
		return s.Rows / s.Distinct // even spread: rows / distinct values
	}
	if n, ok := s.MCV[value]; ok {
		return n // a common value: its real share
	}
	// a rare value: the rows left over, shared by the values left over
	return (s.Rows - s.mcvRows) / (s.Distinct - int64(len(s.MCV)))
}

func plan(est int64) string {
	if est <= loopMaxRows {
		return "nested loop"
	}
	return "hash join"
}

func main() {
	heavy := map[string]int64{"router pool": 5_000_000, "aero WETH/USDC": 1_500_000,
		"uni WETH/cbBTC": 900_000, "topaz pool": 500_000, "small pool": 200_000}
	counts := map[string]int64{"tiny pool": tinyPoolRows}
	var used int64 = tinyPoolRows
	for k, v := range heavy {
		counts[k] = v
		used += v
	}
	// spread the remaining rows over the other pools
	rest := int64(distinct - len(counts))
	left := totalRows - used
	for i := int64(0); i < rest; i++ {
		n := left / rest
		if i < left%rest {
			n++
		}
		counts[fmt.Sprintf("pool-%05d", i)] = n
	}

	// ANALYZE without a histogram: two numbers only
	plain := Stats{Rows: totalRows, Distinct: int64(len(counts))}
	// ANALYZE with a histogram: also the most common values
	type kv struct {
		k string
		v int64
	}
	var all []kv
	for k, v := range counts {
		all = append(all, kv{k, v})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].v > all[j].v })
	hist := plain
	hist.MCV = map[string]int64{}
	for _, e := range all[:mcvSize] {
		hist.MCV[e.k] = e.v
		hist.mcvRows += e.v
	}

	fmt.Printf("rows %d, distinct pools %d, even-spread guess %d\n", plain.Rows, plain.Distinct, plain.estimate("", false))
	for _, tab := range []struct {
		name string
		s    Stats
	}{{"no histogram", plain}, {"with histogram", hist}} {
		fmt.Println(tab.name + ":")
		for _, p := range []string{"router pool", "tiny pool"} {
			e := tab.s.estimate(p, true)
			fmt.Printf("  %-12s estimate %8d  actual %8d  plan %s\n", p, e, counts[p], plan(e))
		}
	}
	e := hist.estimate("", false)
	fmt.Printf("generic plan, WHERE pool = $1: estimate %d, plan %s for every pool\n", e, plan(e))
}
