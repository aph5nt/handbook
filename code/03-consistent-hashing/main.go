// Consistent hashing. Twelve wallet keys, three cache nodes, then a fourth.
// The hash of each key is fixed to the lesson's numbers (0 to 359) so the run
// prints the same moves: 10 of 12 with mod N, 2 of 12 on the ring.
package main

import (
	"fmt"
	"sort"
	"strings"
)

// Key is a wallet with its hash on the 0-359 ring.
type Key struct {
	name string
	hash int
}

var keys = []Key{{"0x18", 15}, {"0x9f", 40}, {"0x2a", 75}, {"0x4c", 100}, {"0x51", 130}, {"0x63", 170},
	{"0x77", 200}, {"0x80", 230}, {"0xa3", 255}, {"0xb6", 280}, {"0xc2", 310}, {"0xe9", 345}}

// ModN picks node hash mod N: remainder 0 is A, 1 is B, 2 is C, 3 is D.
func ModN(hash, n int) string { return string(rune('A' + hash%n)) }

// MovesModN lists the keys whose node changes when N goes from `from` to `to`.
func MovesModN(ks []Key, from, to int) []string {
	var m []string
	for _, k := range ks {
		if ModN(k.hash, from) != ModN(k.hash, to) {
			m = append(m, k.name) // a different node: the next lookup misses
		}
	}
	return m
}

// Point is one place of a node on the ring. A node may have several.
type Point struct {
	node string
	pos  int
}

// Owner walks clockwise from the key to the first node point at or after it.
func Owner(hash int, ring []Point) string {
	sort.Slice(ring, func(i, j int) bool { return ring[i].pos < ring[j].pos })
	for _, p := range ring {
		if p.pos >= hash {
			return p.node
		}
	}
	return ring[0].node // past the last point: wrap around to the first
}

// Count returns how many keys each node owns.
func Count(ks []Key, ring []Point) map[string]int {
	c := map[string]int{}
	for _, k := range ks {
		c[Owner(k.hash, ring)]++
	}
	return c
}

// Moved lists the keys whose owner differs between two rings.
func Moved(ks []Key, before, after []Point) []string {
	var m []string
	for _, k := range ks {
		if Owner(k.hash, before) != Owner(k.hash, after) {
			m = append(m, k.name)
		}
	}
	return m
}

// VirtualRing places each machine at several points, here every 30 degrees.
func VirtualRing(machines []string, perMachine int) []Point {
	var r []Point
	step := 360 / (len(machines) * perMachine)
	for i := 1; i <= len(machines)*perMachine; i++ {
		r = append(r, Point{machines[(i-1)%len(machines)], i * step})
	}
	return r
}

func show(c map[string]int) string {
	var s []string
	for _, n := range []string{"A", "B", "C", "D"} {
		if v, ok := c[n]; ok {
			s = append(s, fmt.Sprintf("%s %d", n, v))
		}
	}
	return strings.Join(s, ", ")
}

func main() {
	fmt.Println("== hash mod N")
	moved := 0
	for _, k := range keys {
		a, b := ModN(k.hash, 3), ModN(k.hash, 4)
		mark := ""
		if a != b {
			moved++
			mark = "moves"
		}
		fmt.Printf("%s hash %3d: mod 3 -> %s, mod 4 -> %s %s\n", k.name, k.hash, a, b, mark)
	}
	fmt.Printf("%d of %d wallets change node (%d by MovesModN)\n", moved, len(keys), len(MovesModN(keys, 3, 4)))

	fmt.Println("== Hash ring")
	r3 := []Point{{"A", 120}, {"B", 240}, {"C", 360}}
	fmt.Printf("3 nodes: %s\n", show(Count(keys, r3)))
	r4 := append(append([]Point{}, r3...), Point{"D", 300})
	fmt.Printf("D joins at 300: %s, moved %v\n", show(Count(keys, r4)), Moved(keys, r3, r4))
	fmt.Printf("virtual nodes, 3 per machine: %s\n", show(Count(keys, VirtualRing([]string{"A", "B", "C", "D"}, 3))))
	fmt.Printf("hot key 0x18 lives on %s alone\n", Owner(15, r4))
	split := []Key{{"0x18#1", 15}, {"0x18#2", 140}, {"0x18#3", 325}}
	var on []string
	for _, k := range split {
		on = append(on, k.name+" -> "+Owner(k.hash, r4))
	}
	fmt.Printf("split with a suffix: %s\n", strings.Join(on, ", "))
}
