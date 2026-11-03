// An in-memory model of join-order search. It needs no database: it scores
// every left-deep order of swaps, pools and tokens with a simple cost model
// (rows read on both sides plus rows produced), once with the estimates the
// planner believed and once with the true sizes.
package main

import (
	"fmt"
	"strings"
)

const (
	swapsTable = 2_000_000_000 // every day of swaps
	swapsDay   = 3_000_000     // the true rows of 1 October
	defaultEst = 1_500_000_000 // the guess for a day partition with no statistics
	pools      = 50_000
	tokens     = 20_000
)

// joinSize: swaps carries both keys (pool, token_in), so any join that has
// swaps on one side yields one row per swap. pools and tokens share no key,
// so joining them alone is a cross product.
func joinSize(left map[string]bool, leftRows int64, next string, size map[string]int64) int64 {
	if left["swaps"] || next == "swaps" {
		return size["swaps"]
	}
	return leftRows * size[next]
}

func cost(order []string, size map[string]int64) int64 {
	in := map[string]bool{order[0]: true}
	rows, total := size[order[0]], int64(0)
	for _, next := range order[1:] {
		out := joinSize(in, rows, next, size)
		total += rows + size[next] + out
		rows = out
		in[next] = true
	}
	return total
}

func perms(a []string) [][]string {
	if len(a) <= 1 {
		return [][]string{append([]string{}, a...)}
	}
	var out [][]string
	for i := range a {
		rest := append(append([]string{}, a[:i]...), a[i+1:]...)
		for _, p := range perms(rest) {
			out = append(out, append([]string{a[i]}, p...))
		}
	}
	return out
}

func choose(title string, est, truth map[string]int64) []string {
	fmt.Println(title)
	var best []string
	var bestCost int64 = -1
	for _, o := range perms([]string{"swaps", "pools", "tokens"}) {
		c := cost(o, est)
		fmt.Printf("  %-20s estimated cost %14d   true cost %14d\n", strings.Join(o, " "), c, cost(o, truth))
		if bestCost < 0 || c < bestCost {
			best, bestCost = o, c
		}
	}
	fmt.Printf("  chosen %v, true cost %d\n", best, cost(best, truth))
	return best
}

func main() {
	truth := map[string]int64{"swaps": swapsDay, "pools": pools, "tokens": tokens}
	fmt.Printf("swaps table %d rows, 1 October %d rows\n", int64(swapsTable), int64(swapsDay))
	choose("With statistics for the day partition:", truth, truth)
	bad := map[string]int64{"swaps": defaultEst, "pools": pools, "tokens": tokens}
	wrong := choose("No statistics for the day partition (guess 1,500,000,000):", bad, truth)
	fmt.Printf("  the wrong order does %d times the work\n", cost(wrong, truth)/cost([]string{"swaps", "pools", "tokens"}, truth))
	hint := []string{"swaps", "pools", "tokens"}
	fmt.Printf("Hint LEADING(swaps pools tokens): no search, true cost %d\n", cost(hint, truth))
}
