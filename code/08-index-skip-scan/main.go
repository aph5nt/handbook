// An in-memory model of an index skip scan: twelve sorted entries for the walk,
// and a page-cost formula for the planner's choice. The formula is a model.
package main

import (
	"fmt"
	"sort"
)

type Entry struct {
	Chain  string
	Height int
}

func skipScan(idx []Entry, h int) (seeks int, kept []Entry) {
	pos := 0
	for pos < len(idx) {
		v := idx[pos].Chain
		seeks++
		// Seek to (v, h): the first entry of chain v above the height.
		pos += sort.Search(len(idx)-pos, func(i int) bool {
			e := idx[pos+i]
			return e.Chain > v || (e.Chain == v && e.Height > h)
		})
		for pos < len(idx) && idx[pos].Chain == v {
			kept = append(kept, idx[pos])
			pos++
		}
		// pos now stands on the next distinct chain.
	}
	return seeks, kept
}

const (
	depth    = 3      // pages from root to leaf
	leaves   = 10_000 // one million entries, 100 per leaf
	matching = 100    // 1 percent of the leaves hold matches
)

func skipCost(distinct int) int { return distinct*depth + matching }

func choose(distinct int) string {
	if skipCost(distinct) < leaves {
		return "skip scan"
	}
	return "sweep"
}

func main() {
	idx := []Entry{
		{"base", 21100500}, {"base", 21300000}, {"base", 21450000}, {"base", 21600000},
		{"ethereum", 21200000}, {"ethereum", 21390000}, {"ethereum", 21410000}, {"ethereum", 21700000},
		{"robinhood", 21000000}, {"robinhood", 21350000}, {"robinhood", 21500000}, {"robinhood", 21800000},
	}
	seeks, kept := skipScan(idx, 21400000)
	fmt.Printf("skip scan over 12 entries: %d seeks, %d entries kept\n", seeks, len(kept))
	for _, e := range kept {
		fmt.Printf("  %s %d\n", e.Chain, e.Height)
	}
	fmt.Printf("cost, 3 chains:       %d x %d + %d = %d pages, sweep %d: %s\n", 3, depth, matching, skipCost(3), leaves, choose(3))
	fmt.Printf("cost, 10,000 tenants: %d x %d + %d = %d pages, sweep %d: %s\n", 10000, depth, matching, skipCost(10000), leaves, choose(10000))
	fmt.Printf("statistics say 10 tenants: planner sees %d pages: %s, real cost %d\n", skipCost(10), choose(10), skipCost(10000))
	fmt.Printf("index on block_height:  1 x %d + %d = %d pages\n", depth, matching, depth+matching)
}
