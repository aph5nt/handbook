// Consistent hashing and virtual nodes: the story of the lesson
// 12-consistent-hashing.html, replayed. Sixteen addresses sit on a ring of 360
// points. Nodes join and leave, with and without virtual nodes.
package main

import (
	"fmt"
	"sort"
)

// Keys are the story's sixteen addresses, as points on the ring.
var Keys = []int{10, 30, 55, 80, 100, 130, 150, 175, 200, 230, 250, 265, 300, 320, 345, 5}

// Point is one place on the ring that a node owns.
type Point struct {
	Node string
	At   int
}

// Owner walks clockwise from the key to the first point. Past the top of the
// ring it wraps round to the lowest point.
func Owner(key int, ring []Point) string {
	s := append([]Point{}, ring...)
	sort.Slice(s, func(i, j int) bool { return s[i].At < s[j].At })
	for _, p := range s {
		if p.At >= key {
			return p.Node
		}
	}
	return s[0].Node
}

// Moved counts the keys whose owner changed, and where they went.
func Moved(before, after []Point) (int, map[string]int) {
	to := map[string]int{}
	n := 0
	for _, k := range Keys {
		if o := Owner(k, after); o != Owner(k, before) {
			n++
			to[o]++
		}
	}
	return n, to
}

// Count is the number of keys each node owns.
func Count(ring []Point) map[string]int {
	c := map[string]int{}
	for _, k := range Keys {
		c[Owner(k, ring)]++
	}
	return c
}

// Without returns the ring with every point of one node removed.
func Without(ring []Point, node string) []Point {
	out := []Point{}
	for _, p := range ring {
		if p.Node != node {
			out = append(out, p)
		}
	}
	return out
}

func main() {
	fmt.Println("== ring: add and remove a node")
	n3 := []Point{{"A", 40}, {"B", 160}, {"C", 280}}
	fmt.Println("1 nodes at A 40, B 160, C 280")
	fmt.Println("2 sixteen keys on the same ring")
	fmt.Println("3 key 100 walks clockwise to", Owner(100, n3), " counts", Count(n3))
	n4 := append(append([]Point{}, n3...), Point{"D", 220})
	m, to := Moved(n3, n4)
	fmt.Println("4 add D at 220:", m, "of 16 keys moved, to", to, " counts", Count(n4))
	modMoved := 0
	for _, k := range Keys {
		if k%3 != k%4 {
			modMoved++
		}
	}
	fmt.Println("  hash mod 3 to mod 4 would move", modMoved, "of 16")
	n3b := Without(n4, "B")
	m, to = Moved(n4, n3b)
	fmt.Println("5 remove B:", m, "keys moved, to", to, " counts", Count(n3b))
	v3 := []Point{{"A", 40}, {"B", 160}, {"C", 280}, {"A", 170}, {"B", 290}, {"C", 50}, {"A", 300}, {"B", 60}, {"C", 190}}
	fmt.Println("6 three points per node, counts", Count(v3))

	fmt.Println("== vnode: no virtual nodes")
	bad := []Point{{"A", 20}, {"B", 50}, {"C", 80}}
	fmt.Println("1 A 20, B 50, C 80: counts", Count(bad))
	m, to = Moved(bad, Without(bad, "A"))
	fmt.Println("2 A fails:", m, "keys moved, to", to, " counts", Count(Without(bad, "A")))
	vb := []Point{{"A", 20}, {"B", 50}, {"C", 80}, {"A", 140}, {"B", 200}, {"C", 280}, {"A", 240}, {"B", 310}, {"C", 170}, {"A", 330}, {"B", 110}, {"C", 340}}
	fmt.Println("3 four points per node, counts", Count(vb))
	m, to = Moved(vb, Without(vb, "A"))
	fmt.Println("4 A fails:", m, "keys moved, to", to, " counts", Count(Without(vb, "A")))
	fmt.Println("5 hot key 200 is owned by", Owner(200, vb), "alone, on any ring")
}
