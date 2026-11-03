// Consistent hashing versus hash mod N, and why neither helps one hot key.
package main

import (
	"fmt"
	"hash/fnv"
	"sort"
)

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	// FNV clusters on short similar strings, so mix the bits.
	x := h.Sum32()
	x ^= x >> 16
	x *= 0x85ebca6b
	x ^= x >> 13
	x *= 0xc2b2ae35
	x ^= x >> 16
	return x
}

// Ring places each node at several points on a circle.
type Ring struct {
	points []uint32
	owner  map[uint32]string
}

func NewRing(nodes []string, virtual int) *Ring {
	r := &Ring{owner: map[uint32]string{}}
	for _, n := range nodes {
		for v := 0; v < virtual; v++ {
			p := hash(fmt.Sprintf("%s#%d", n, v))
			r.points = append(r.points, p)
			r.owner[p] = n
		}
	}
	sort.Slice(r.points, func(i, j int) bool { return r.points[i] < r.points[j] })
	return r
}

// Node walks clockwise from the key to the first point.
func (r *Ring) Node(key string) string {
	h := hash(key)
	i := sort.Search(len(r.points), func(i int) bool { return r.points[i] >= h })
	if i == len(r.points) {
		i = 0 // past the top: wrap around the circle
	}
	return r.owner[r.points[i]]
}

func modN(key string, n int) int { return int(hash(key) % uint32(n)) }

func main() {
	keys := make([]string, 100_000)
	for i := range keys {
		keys[i] = fmt.Sprint("risk:", i)
	}
	three := []string{"node1", "node2", "node3"}
	four := append(append([]string{}, three...), "node4")

	movedMod := 0
	for _, k := range keys {
		if modN(k, 3) != modN(k, 4) {
			movedMod++
		}
	}
	fmt.Printf("hash mod N, 3 to 4 nodes: %.0f%% of keys move\n", 100*float64(movedMod)/float64(len(keys)))

	for _, v := range []int{1, 100} {
		a, b := NewRing(three, v), NewRing(four, v)
		moved := 0
		load := map[string]int{}
		for _, k := range keys {
			if a.Node(k) != b.Node(k) {
				moved++
			}
			load[b.Node(k)]++
		}
		fmt.Printf("ring with %3d points per node: %.0f%% of keys move, load after: %v\n",
			v, 100*float64(moved)/float64(len(keys)), load)
	}

	r := NewRing(four, 100)
	fmt.Printf("\nthe hot key price:native lives on %s, 4 nodes or 40\n", r.Node("price:native"))
	const services, readsPerSec = 20, 100_000
	fmt.Printf("no fix: %d reads/s on one node\n", readsPerSec)
	fixA := float64(services) / 10 // fix A: a local map in each service, read from it for 10 seconds
	fmt.Printf("fix A, local cache 10 s, %d services: %.0f reads/s on that node, the copy may be 10 s old\n", services, fixA)
	nodes := map[string]int{}
	for i := 0; i < readsPerSec; i++ {
		nodes[r.Node(fmt.Sprintf("price:native#%d", i%3+1))]++
	}
	fmt.Printf("fix B, 3 suffixed copies: reads/s per node %v, every write must reach all 3\n", nodes)
}
