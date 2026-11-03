// Data lineage: edges recorded from real runs, with the columns they
// carry. A column-level walk lists every output a bug in amount_in
// affects, and spares the graph layer, which reads only trader.
package main

import (
	"fmt"
	"sort"
)

// Edge says: job output To reads these columns of From.
type Edge struct {
	From, To string
	Columns  []string // empty means all columns of From
}

var edges = []Edge{
	{"chain node", "indexer decoder", nil},
	{"indexer decoder", "swap facts", []string{"amount_in", "trader"}},
	{"swap facts", "swaps view", []string{"amount_in"}},
	{"swap facts", "graph layer", []string{"trader"}},
	{"swap facts", "daily volume rollup", []string{"amount_in"}},
	{"swaps view", "public API", nil},
	{"daily volume rollup", "grafana board", nil},
	{"daily volume rollup", "customer report", nil},
}

func carries(e Edge, col string) bool {
	if e.Columns == nil {
		return true
	}
	for _, c := range e.Columns {
		if c == col {
			return true
		}
	}
	return false
}

// Downstream walks from a table along edges that carry the broken column.
// Past the first hop the whole output is derived from it, so any edge counts.
func Downstream(start, col string) []string {
	seen := map[string]bool{}
	var walk func(node string, first bool)
	walk = func(node string, first bool) {
		for _, e := range edges {
			if e.From != node || seen[e.To] {
				continue
			}
			if first && !carries(e, col) {
				continue // column level: this reader never sees the column
			}
			seen[e.To] = true
			walk(e.To, false)
		}
	}
	walk(start, true)
	return keys(seen) // 5 outputs for amount_in
}

// keys lists a set in sorted order.
func keys(set map[string]bool) []string {
	out := []string{}
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Upstream walks back from an output to its sources.
func Upstream(node string) []string {
	path := []string{node}
	for {
		found := false
		for _, e := range edges {
			if e.To == node {
				path = append(path, e.From)
				node, found = e.From, true
				break
			}
		}
		if !found {
			return path
		}
	}
}

func main() {
	affected := Downstream("swap facts", "amount_in")
	fmt.Printf("amount_in bug, rebuild %d outputs: %v\n", len(affected), affected)
	fmt.Println("graph layer affected:", contains(affected, "graph layer"))
	fmt.Println("upstream of the grafana board:", Upstream("grafana board"))

	// without lineage: the team fixes what it remembers
	remembered := []string{"public API", "swaps view"}
	missed := []string{}
	for _, a := range affected {
		if !contains(remembered, a) {
			missed = append(missed, a)
		}
	}
	fmt.Printf("no lineage: fixed %d, missed %d: %v\n", len(remembered), len(missed), missed) // 2, 3
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
