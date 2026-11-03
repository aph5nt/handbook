// Three join algorithms on the lesson's own keys. Each counts its work.
package main

import (
	"fmt"
	"sort"
	"strconv"
)

// swaps name a pool each; pools are the inner table.
var swaps = []string{"p2", "p4", "p1", "p2", "p3"}
var pools = []string{"p3", "p1", "p5", "p2"}

// nestedLoop compares every swap with every pool.
func nestedLoop() {
	cmp, found := 0, 0
	for n, s := range swaps { // outer loop: each swap once
		for _, p := range pools { // inner loop: every pool, every time
			cmp++
			if s == p {
				found++
			}
		}
		fmt.Printf("nested loop: swap %d (%s) done, comparisons %d\n", n+1, s, cmp)
	}
	fmt.Printf("nested loop: %d matches, %d comparisons = %d x %d\n", found, cmp, len(swaps), len(pools))
}

// indexedLoop looks each swap up in an index on pools.pool.
func indexedLoop() {
	index := map[string]bool{} // stands for the B-tree on pools.pool
	for _, p := range pools {
		index[p] = true
	}
	lookups, found := 0, 0
	for _, s := range swaps {
		lookups++ // one root-to-leaf walk per swap
		if index[s] {
			found++
		}
	}
	fmt.Printf("indexed loop: %d matches, %d lookups\n", found, lookups)
	fmt.Printf("no index, wrong estimate: %d x %d = %d comparisons\n", 10000, 30000000, 10000*30000000)
}

// bucket is the hash function: the number in the key, modulo 4.
func bucket(key string) int {
	n, _ := strconv.Atoi(key[1:])
	return n % 4
}

// hashJoin builds a table on pools, then probes it with each swap.
func hashJoin() {
	work, found := 0, 0
	table := map[int][]string{}
	for _, p := range pools { // build: read the small side once
		table[bucket(p)] = append(table[bucket(p)], p)
		work++
	}
	fmt.Printf("hash build: bucket 1 %v, bucket 2 %v, bucket 3 %v, work %d\n", table[1], table[2], table[3], work)
	for n, s := range swaps { // probe: stream the big side past
		work++
		hit := false
		for _, p := range table[bucket(s)] { // only the keys in one bucket
			if p == s {
				hit = true
			}
		}
		if hit {
			found++
		}
		fmt.Printf("hash probe: swap %d (%s) -> bucket %d %v, match %v, work %d\n", n+1, s, bucket(s), table[bucket(s)], hit, work)
	}
	fmt.Printf("hash join: %d matches, work %d = %d inserts + %d probes\n", found, work, len(pools), len(swaps))
	fmt.Printf("wrong build side: %d rows must fit in memory\n", 30000000)
}

// mergeJoin sorts both sides, then walks two pointers.
func mergeJoin() {
	o := append([]string{}, swaps...)
	in := append([]string{}, pools...)
	sort.Strings(o) // costs about n log n
	sort.Strings(in)
	fmt.Printf("merge sorted: swaps %v, pools %v\n", o, in)
	a, b, cmp, found := 0, 0, 0, 0
	for a < len(o) && b < len(in) {
		cmp++
		switch {
		case o[a] == in[b]: // keys equal: match, move the swap pointer
			found++
			fmt.Printf("merge: %s = %s match, comparisons %d\n", o[a], in[b], cmp)
			a++
		case o[a] < in[b]: // swap smaller: move the swap pointer
			fmt.Printf("merge: %s < %s move swap pointer, comparisons %d\n", o[a], in[b], cmp)
			a++
		default: // pool smaller: move the pool pointer
			fmt.Printf("merge: %s > %s move pool pointer, comparisons %d\n", o[a], in[b], cmp)
			b++
		}
	}
	fmt.Printf("merge join: %d matches, %d comparisons\n", found, cmp)
}

func main() {
	nestedLoop()
	indexedLoop()
	hashJoin()
	mergeJoin()
}
