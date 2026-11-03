// Hedged requests: the story of the lesson 12-hedged-requests.html.
// An MCP proxy reads from two graph replicas. 90 of 100 reads take 20 ms,
// 5 take 50 ms, and 5 hit a garbage-collection pause of 2 s. A hedge sent at
// the p95 wait cuts that tail for about 5 percent extra load.
package main

import (
	"errors"
	"fmt"
	"sort"
)

const (
	hedgeAfter = 50 // ms: the p95 wait
	replica2   = 20 // ms: replica 2 is not paused
)

// Proxy counts how much work it sends and caps hedges with a budget.
type Proxy struct {
	Reads, Hedges int
	Budget        float64 // most hedges as a share of reads
}

// Read returns when the first copy answers. first is replica 1's time.
func (p *Proxy) Read(first int, idempotent bool) (int, error) {
	p.Reads++
	if first <= hedgeAfter {
		return first, nil // answered before the p95 wait: no hedge
	}
	if !idempotent {
		return first, errors.New("not idempotent: never hedge, wait")
	}
	if float64(p.Hedges) >= p.Budget*float64(p.Reads)+1 { // +1 lets the first one through
		return first, errors.New("hedge budget spent: wait")
	}
	p.Hedges++                      // send the same read to replica 2
	second := hedgeAfter + replica2 // 50 + 20 = 70 ms
	if second < first {
		return second, nil // take the first answer, cancel replica 1
	}
	return first, nil // replica 1 won after all, cancel replica 2
}

func p99(ms []int) int {
	s := append([]int(nil), ms...)
	sort.Ints(s)
	return s[len(s)*99/100-1]
}

func main() {
	var reads []int
	for i := 0; i < 100; i++ {
		switch {
		case i%20 == 7:
			reads = append(reads, 2000) // replica 1 in a pause
		case i%20 == 13:
			reads = append(reads, 50)
		default:
			reads = append(reads, 20)
		}
	}
	fmt.Printf("no hedge: p99 %d ms\n", p99(reads))

	p := &Proxy{Budget: 0.05}
	var got []int
	for _, ms := range reads {
		t, _ := p.Read(ms, true)
		got = append(got, t)
	}
	fmt.Printf("hedge after %d ms: p99 %d ms, %d hedges for %d reads = %d%% extra load\n",
		hedgeAfter, p99(got), p.Hedges, p.Reads, p.Hedges*100/p.Reads)

	fmt.Println("== Hedge everything")
	fmt.Printf("every read sent to both: %d requests for %d reads = 200%% load\n", 2*len(reads), len(reads))
	_, err := p.Read(2000, false)
	fmt.Println("a slow write that adds a label:", err)
	labels := map[string]int{}
	for _, replica := range []string{"replica 1", "replica 2"} {
		labels["0x9f..0071"]++ // without the check, each replica applies the write
		fmt.Println("  label added by", replica)
	}
	fmt.Printf("hedged without the check: the address has the label %d times\n", labels["0x9f..0071"])
}
