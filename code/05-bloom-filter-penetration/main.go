// A Bloom filter in front of the cache stops lookups for addresses that
// do not exist. Part 1 replays the lesson's 16-bit filter with its hash
// values. Part 2 runs a real filter with FNV hashes at a realistic size.
package main

import (
	"fmt"
	"hash/fnv"
)

type Bloom struct {
	bits []bool
	pos  func(key string) []int // the k bit positions of a key
}

// Add sets every bit of the key to one.
func (b *Bloom) Add(key string) {
	for _, p := range b.pos(key) {
		b.bits[p] = true
	}
}

// MayContain is false only when a bit is zero: then the key was never added.
func (b *Bloom) MayContain(key string) bool {
	for _, p := range b.pos(key) {
		if !b.bits[p] {
			return false // one zero bit is proof: definitely not
		}
	}
	return true // all ones: maybe
}

// fnvPositions derives k positions from two FNV hashes (double hashing).
func fnvPositions(m, k int) func(string) []int {
	return func(key string) []int {
		h1 := fnv.New64a()
		h1.Write([]byte(key))
		a := h1.Sum64()
		h2 := fnv.New64()
		h2.Write([]byte(key))
		c := h2.Sum64() | 1
		out := make([]int, k)
		for i := range out {
			out[i] = int((a + uint64(i)*c) % uint64(m))
		}
		return out
	}
}

func main() {
	// the lesson's hash values, so the numbers match the animation
	lesson := map[string][]int{
		"0x18..cb24": {2, 7, 13}, "0x77..30ff": {4, 7, 11},
		"0xab..9f10": {2, 5, 11}, "0xcd..0e42": {2, 4, 13}, "0x5e..aa01": {3, 9, 14},
	}
	b := &Bloom{bits: make([]bool, 16), pos: func(k string) []int { return lesson[k] }}
	graph := map[string]bool{}
	graphReads := 0
	cacheThenGraph := func(addr string) string {
		graphReads++
		if graph[addr] {
			return "maybe -> graph: found"
		}
		return "maybe -> graph: not found (a false positive)"
	}
	lookup := func(addr string) string {
		if !b.MayContain(addr) {
			return "not found" // the graph is not asked
		}
		return cacheThenGraph(addr)
	}
	for _, addr := range []string{"0x18..cb24", "0x77..30ff"} {
		graph[addr] = true // graph sync: write the address and add it in the same step
		b.Add(addr)
	}
	for _, a := range []string{"0xab..9f10", "0x77..30ff", "0xcd..0e42"} {
		fmt.Printf("%s bits %v: %s\n", a, lesson[a], lookup(a))
	}
	fmt.Printf("graph reads: %d of 3 lookups\n", graphReads)

	graph["0x5e..aa01"] = true // the graph sync writes a new address
	fmt.Printf("new 0x5e..aa01, not added to the filter: %s\n", lookup("0x5e..aa01"))
	b.Add("0x5e..aa01") // the fix: add on write
	fmt.Printf("after Add on write: %s\n", lookup("0x5e..aa01"))

	// part 2: a real filter, 10 bits per address, 7 hashes
	const n, m, k = 100_000, 1_000_000, 7
	real := &Bloom{bits: make([]bool, m), pos: fnvPositions(m, k)}
	for i := 0; i < n; i++ {
		real.Add(fmt.Sprintf("known-%d", i))
	}
	fp := 0
	for i := 0; i < n; i++ {
		if real.MayContain(fmt.Sprintf("fake-%d", i)) {
			fp++
		}
	}
	fmt.Printf("\nreal filter: %d addresses in %d bits (%d KB), %d hashes\n", n, m, m/8/1024, k)
	fmt.Printf("%d random fakes: %d said maybe (%.2f%%), the rest never reach the graph\n", n, fp, 100*float64(fp)/n)
}
