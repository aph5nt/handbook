// Sharding and rebalancing: the story of the lesson
// 12-sharding-and-rebalance.html, replayed. Twelve users are placed on shards
// by hash modulo N, then on a ring, then one hot wallet overloads one shard.
package main

import "fmt"

const names = "ABCDE"

// The story's twelve users: hash values for modulo, and ring positions in
// degrees (the same hash bent into a circle).
var hash = []int{19, 65, 30, 68, 3, 57, 10, 26, 60, 69, 35, 64}
var angle = []int{10, 50, 85, 105, 135, 155, 185, 230, 260, 290, 320, 350}

// Modulo places a key on shard hash mod n.
func Modulo(h, n int) int { return h % n }

// Point is one shard (or one virtual node of it) on the ring.
type Point struct{ At, Shard int }

// Owner walks clockwise from the key to the first shard point.
func Owner(a int, ring []Point) int {
	best, dist := -1, 361
	for _, p := range ring {
		d := (p.At - a + 360) % 360
		if d < dist {
			best, dist = p.Shard, d
		}
	}
	return best
}

// Moved counts keys whose shard differs between two placements.
func Moved(before, after func(k int) int) int {
	n := 0
	for k := range hash {
		if before(k) != after(k) {
			n++
		}
	}
	return n
}

// Arcs is how many degrees of the ring each shard owns: the arc before each
// of its points.
func Arcs(ring []Point, shards int) []int {
	arc := make([]int, shards)
	for a := 0; a < 360; a++ {
		arc[Owner(a, ring)]++
	}
	return arc
}

func counts(place func(k int) int, shards int) []int {
	c := make([]int, shards)
	for k := range hash {
		c[place(k)]++
	}
	return c
}

// Load sums requests per second per shard. A split key spreads over parts.
func Load(ring []Point, shards int, hot int, split []int) []int {
	load := make([]int, shards)
	for s := range load {
		load[s] = 400 / shards // the ordinary users, spread evenly
	}
	if hot < 0 {
		return load
	}
	if split == nil {
		load[Owner(angle[hot], ring)] += 400 // one key, one shard
		return load
	}
	for _, a := range split {
		load[Owner(a, ring)] += 400 / len(split) // key#0..key#3 on four shards
	}
	return load
}

func main() {
	fmt.Println("== mod: hash modulo N")
	fmt.Printf("1 user 1: hash %d mod 4 = %d, shard %c\n", hash[0], Modulo(hash[0], 4), names[Modulo(hash[0], 4)])
	fmt.Println("2 users per shard A-D:", counts(func(k int) int { return Modulo(hash[k], 4) }, 4))
	m := Moved(func(k int) int { return Modulo(hash[k], 4) }, func(k int) int { return Modulo(hash[k], 5) })
	fmt.Printf("3 add shard E, mod 5: %d of 12 users moved (E needs only 12/5 = 2)\n", m)

	fmt.Println("== ring: consistent hashing")
	four := []Point{{30, 0}, {120, 1}, {210, 2}, {300, 3}}
	five := append([]Point{{170, 4}}, four...)
	fmt.Printf("1 user 7 at %d degrees: next shard clockwise is %c\n", angle[6], names[Owner(angle[6], four)])
	m = Moved(func(k int) int { return Owner(angle[k], four) }, func(k int) int { return Owner(angle[k], five) })
	fromC := Moved(func(k int) int { return Owner(angle[k], four) }, func(k int) int {
		if Owner(angle[k], four) != 2 {
			return Owner(angle[k], four) // only count keys that left C
		}
		return Owner(angle[k], five)
	})
	fmt.Printf("2 add E at 170: %d of 12 users moved, %d of them from C\n", m, fromC)
	fmt.Println("3 users per shard A-E, one point each:", counts(func(k int) int { return Owner(angle[k], five) }, 5),
		" degrees owned:", Arcs(five, 5))
	vnodes := append([]Point{{70, 2}, {245, 0}, {335, 4}, {197, 3}, {275, 1}}, five...)
	fmt.Println("4 users per shard A-E, two points each:", counts(func(k int) int { return Owner(angle[k], vnodes) }, 5),
		" degrees owned:", Arcs(vnodes, 5))

	fmt.Println("== hot: one key gets most of the traffic (requests per second)")
	fmt.Println("1 even traffic A-D:", Load(four, 4, -1, nil))
	fmt.Println("2 user 7, the hot wallet, on shard", string(names[Owner(angle[6], four)]), Load(four, 4, 6, nil))
	fmt.Println("3 add shard E:", Load(five, 5, 6, nil))
	fmt.Println("4 split user 7 into 4 parts:", Load(four, 4, 6, []int{20, 100, 185, 280}))
}
