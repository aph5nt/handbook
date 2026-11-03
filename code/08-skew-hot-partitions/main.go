// An in-memory model of a shuffle join: rows hashed to four nodes, with a
// counter per node. The swaps are generated, not read from a database.
package main

import (
	"fmt"
	"hash/fnv"
)

const nodes = 4

// node is hash(key) mod 4, the rule a shuffle join uses.
func hash(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	v := h.Sum32()
	return v ^ (v >> 16)
}

func node(key string) int { return int(hash(key) % nodes) }

func shares(counts [nodes]int) [nodes]float64 {
	total := 0
	for _, c := range counts {
		total += c
	}
	var out [nodes]float64
	for i, c := range counts {
		out[i] = 100 * float64(c) / float64(total)
	}
	return out
}

// findRouter picks the example router: on node 0, with four salted keys that
// land on four different nodes. A salt does not promise that by itself.
func findRouter() string {
	for n := 0; ; n++ {
		r := fmt.Sprintf("0x%04x", n)
		if node(r) != 0 {
			continue
		}
		seen := map[int]bool{}
		for s := 0; s < nodes; s++ {
			seen[node(fmt.Sprintf("%s#%d", r, s))] = true
		}
		if len(seen) == nodes {
			return r
		}
	}
}

func main() {
	const smallSenders, rowsEach = 100_000, 10
	router := findRouter()
	var small, skew, salted [nodes]int
	for s := 0; s < smallSenders; s++ {
		n := node(fmt.Sprintf("sender-%d", s))
		small[n] += rowsEach
		skew[n] += rowsEach
		salted[n] += rowsEach
	}
	routerRows := smallSenders * rowsEach // half of all swaps
	skew[node(router)] += routerRows
	fmt.Printf("small senders alone, rows per node: %v\n", small)
	fmt.Printf("router %s hashes to node %d\n", router, node(router)+1)
	fmt.Printf("with the router, share per node:    %.1f\n", shares(skew))

	for i := 0; i < routerRows; i++ {
		salted[node(fmt.Sprintf("%s#%d", router, i%nodes))]++
	}
	for s := 0; s < nodes; s++ {
		k := fmt.Sprintf("%s#%d", router, s)
		fmt.Printf("  salted key %s goes to node %d\n", k, node(k)+1)
	}
	fmt.Printf("salted, share per node:             %.1f\n", shares(salted))

	fmt.Println("Hot partition: 1,000,000 writes today, spread over the buckets of today")
	for _, buckets := range []int{4, 8} {
		count := make([]int, buckets)
		for i := 0; i < 1_000_000; i++ {
			count[hash(fmt.Sprintf("tx-%d", i))%uint32(buckets)]++
		}
		top := 0
		for _, c := range count {
			if c > top {
				top = c
			}
		}
		fmt.Printf("  %d buckets: busiest bucket holds %d writes\n", buckets, top)
	}
}
