// Histograms: the story of the lesson 12-histograms.html.
// Three copies of an MCP proxy count query times into buckets. Summing the
// buckets and reading once gives the true p99. Averaging three p99s lies.
package main

import "fmt"

// Edges are the bucket upper bounds in milliseconds.
var Edges = []int{50, 100, 250, 500, 1000, 2500}

// Hist is one counter per bucket.
type Hist []int

// Observe adds 1 to the first bucket the time fits.
func (h Hist) Observe(ms int) {
	for i, e := range Edges {
		if ms <= e {
			h[i]++
			return
		}
	}
}

// Merge adds bucket by bucket. Counts add up, so this is exact.
func Merge(hs ...Hist) Hist {
	m := make(Hist, len(Edges))
	for _, h := range hs {
		for i, c := range h {
			m[i] += c
		}
	}
	return m
}

// Quantile walks from the fast end until the running count reaches q of the
// total, and returns that bucket's upper edge.
func Quantile(q float64, h Hist) int {
	total := 0
	for _, c := range h {
		total += c
	}
	target := int(q*float64(total) + 0.999999) // round up: the 990th of 1,000
	run := 0
	for i, c := range h {
		run += c
		if run >= target {
			return Edges[i]
		}
	}
	return Edges[len(Edges)-1]
}

func main() {
	a := Hist{700, 250, 46, 3, 1, 0}
	b := Hist{680, 270, 46, 3, 1, 0}
	c := Hist{400, 300, 100, 50, 50, 100}
	slow := make(Hist, len(Edges))
	slow.Observe(1800) // a cold-partition query of 1.8 s
	fmt.Printf("a 1,800 ms query lands in %v (the under-2,500 bucket)\n", slow)
	fmt.Printf("copy A p99 under %d ms, copy B under %d ms, copy C under %d ms\n",
		Quantile(0.99, a), Quantile(0.99, b), Quantile(0.99, c))
	m := Merge(a, b, c)
	fmt.Printf("merged %v: true p99 under %d ms\n", m, Quantile(0.99, m))
	avg := (Quantile(0.99, a) + Quantile(0.99, b) + Quantile(0.99, c)) / 3
	fmt.Printf("average of the three p99s: %d ms (too low)\n", avg)

	cs := Hist{50, 30, 8, 4, 3, 5} // next day: copy C serves only 100 queries
	avg = (Quantile(0.99, a) + Quantile(0.99, b) + Quantile(0.99, cs)) / 3
	m = Merge(a, b, cs)
	fmt.Printf("next day: average still %d ms, merged %v: true p99 under %d ms (average far too high)\n",
		avg, m, Quantile(0.99, m))
}
