// Replays the cardinality lesson: labels multiply, bounded labels stay flat
// with traffic, one unbounded label takes the whole store down.
package main

import "fmt"

const storeLimit = 1_000_000 // series this Prometheus can hold

// Label is one metric label and how many values it can take.
type Label struct {
	Name   string
	Values int
}

// Cardinality is the product of the value counts.
func Cardinality(labels []Label) int {
	n := 1
	for _, l := range labels {
		n *= l.Values
	}
	return n
}

// Memory is the share of the store's memory: a base plus the series.
func Memory(series int) float64 {
	return 0.05 + float64(series)/storeLimit
}

// Boot loads the series from disk on start. Too many, and it dies; a restart loads them again.
func Boot(series, restarts int) string {
	if Memory(series) < 1 {
		return "healthy"
	}
	for r := 0; r < restarts; r++ {
		fmt.Printf("  restart %d: load %d series, out of memory\n", r+1, series)
	}
	return "restart loop: alerts have no data"
}

// Observe counts the distinct series that one stream of requests makes.
func Observe(requests int, labelsOf func(int) string) int {
	seen := map[string]bool{}
	for r := 0; r < requests; r++ {
		seen[labelsOf(r)] = true
	}
	return len(seen)
}

// Relabel is the scrape rule: it drops a label before the store sees it.
func Relabel(labels []Label, drop string) []Label {
	var kept []Label
	for _, l := range labels {
		if l.Name != drop {
			kept = append(kept, l)
		}
	}
	return kept
}

// Admit is the cap per metric: a scrape over the limit is refused, the store lives.
func Admit(series, limit int) bool {
	return series <= limit
}

func main() {
	var ls []Label
	fmt.Println("no labels:", Cardinality(ls))
	for _, l := range []Label{{"lane", 2}, {"endpoint", 10}, {"status", 5}, {"bucket", 10}} {
		ls = append(ls, l)
		fmt.Printf("add %-8s: %5d series, memory %.0f%%\n", l.Name, Cardinality(ls), Memory(Cardinality(ls))*100)
	}
	bounded := func(r int) string {
		return fmt.Sprintf("%d,%d,%d,%d", r%2, r/2%10, r/20%5, r/100%10)
	}
	fmt.Println("1,000,000 requests make", Observe(1_000_000, bounded), "series")

	base := ls[:3]
	fmt.Println("\nstart:", Cardinality(base))
	for _, addresses := range []int{5_000, 300_000} {
		w := append(append([]Label{}, base...), Label{"address", addresses})
		n := Cardinality(w)
		fmt.Printf("address %7d values: %d series, memory %.0f%%, %s\n", addresses, n, Memory(n)*100, Boot(n, 2))
		fmt.Println("  cap of 100,000 per metric admits it:", Admit(n, 100_000))
	}
	w := append(append([]Label{}, base...), Label{"address", 300_000})
	fixed := Relabel(w, "address")
	fmt.Println("after the scrape rule drops address:", Cardinality(fixed), "series,", Boot(Cardinality(fixed), 2))
}
