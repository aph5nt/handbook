// Cache sizing: the hit-rate curve of a skewed workload, and thrashing
// when the cache is one key smaller than a loop's working set.
package main

import (
	"fmt"
	"math/rand"
	"slices"
)

const n = 20 // keys in the model

// weight of key k (1 = hottest): reads fall off as 1/k, a Zipf-like skew.
func weight(k int) float64 { return 1 / float64(k) }

// modelHitRate: a cache holding the s hottest keys serves their share of reads.
func modelHitRate(s int) float64 {
	var top, all float64
	for k := 1; k <= n; k++ {
		all += weight(k)
		if k <= s {
			top += weight(k)
		}
	}
	return top / all
}

// lruHits replays a trace of keys against an LRU cache of the given size.
func lruHits(trace []int, size int) int {
	var c []int // c[0] = least recently used
	hits := 0
	for _, k := range trace {
		if i := slices.Index(c, k); i >= 0 {
			hits++
			c = append(c[:i], c[i+1:]...)
		} else if len(c) == size {
			c = c[1:] // evict the least recently used
		}
		c = append(c, k)
	}
	return hits
}

func main() {
	fmt.Println("== the hit-rate curve (model: cache the s hottest keys) ==")
	for _, s := range []int{2, 5, 10, 20} {
		fmt.Printf("  %2d keys: %3.0f%% of reads hit\n", s, 100*modelHitRate(s))
	}

	// "measure, then size": record a trace and replay it against several sizes
	r := rand.New(rand.NewSource(1))
	cum := make([]float64, n+1)
	for k := 1; k <= n; k++ {
		cum[k] = cum[k-1] + weight(k)
	}
	recordedTrace := make([]int, 100_000)
	for i := range recordedTrace {
		x := r.Float64() * cum[n]
		k := 1
		for cum[k] < x {
			k++
		}
		recordedTrace[i] = k
	}
	fmt.Println("== replay of a recorded trace of 100,000 reads, LRU ==")
	for _, s := range []int{2, 5, 10, 20} {
		hits := lruHits(recordedTrace, s) // replay real keys at each size
		fmt.Printf("  %2d slots: %3.0f%% hit\n", s, 100*float64(hits)/float64(len(recordedTrace)))
	}

	fmt.Println("== a loop over keys 1 to 6, read 100 times ==")
	var loop []int
	for i := 0; i < 100; i++ {
		for k := 1; k <= 6; k++ {
			loop = append(loop, k)
		}
	}
	loop1to6 := loop
	h5 := lruHits(loop1to6, 5) // 0 of 600: every read misses
	h6 := lruHits(loop1to6, 6) // 594 of 600
	fmt.Printf("  LRU with 5 slots: %d hits of %d reads\n", h5, len(loop1to6))
	fmt.Printf("  LRU with 6 slots: %d hits of %d reads\n", h6, len(loop1to6))
}
