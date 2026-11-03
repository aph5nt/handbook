// Command dynamo replays the Dynamo quorum lesson: N = 3, W = 2, R = 2 on a
// preference list, a slow node, a read that overlaps the write, a sloppy
// quorum with a hint, a network split with siblings, and the union merge
// that brings a removed item back.
package main

import (
	"fmt"
	"sort"
	"strings"
)

type Version struct {
	Items []string
	Clock map[string]int
}

const N, W, R = 3, 2, 2

type Node struct {
	Name   string
	Up     bool
	Slow   bool
	Copies map[string]Version
	Hints  []string // owners this node holds writes for
}

func Put(ring []*Node, key string, v Version) (acks int, hint string) {
	written := 0
	for i := 0; i < len(ring) && written < N; i++ {
		n := ring[i]
		if !n.Up {
			if i < N {
				hint = n.Name
			}
			continue
		}
		n.Copies[key] = v
		written++
		if !n.Slow {
			acks++
		}
	}
	return acks, hint
}

// Includes says whether clock a has seen everything in clock b.
func Includes(a, b map[string]int) bool {
	for k, v := range b {
		if a[k] < v {
			return false
		}
	}
	return true
}

func Get(copies []Version) []Version {
	asked := copies[:R]
	if Includes(asked[0].Clock, asked[1].Clock) {
		return asked[:1]
	}
	if Includes(asked[1].Clock, asked[0].Clock) {
		return asked[1:]
	}
	return asked
}

func MergeUnion(a, b Version) []string {
	seen := map[string]bool{}
	for _, x := range append(append([]string{}, a.Items...), b.Items...) {
		seen[x] = true
	}
	var out []string
	for x := range seen {
		out = append(out, x)
	}
	sort.Strings(out)
	return out
}

func main() {
	mk := func(n string) *Node { return &Node{Name: n, Up: true, Copies: map[string]Version{}} }
	B, C, D, E := mk("B"), mk("C"), mk("D"), mk("E")
	ring := []*Node{B, C, D, E}
	fmt.Println("write tab")
	fmt.Printf("step 1: N=%d W=%d R=%d, R+W=%d > N: reads overlap writes\n", N, W, R, R+W)
	D.Slow = true
	cart := Version{[]string{"headphones"}, map[string]int{"B": 1}}
	acks, _ := Put(ring, "cart", cart)
	fmt.Printf("step 2-3: add headphones: %d fast answers, W=%d met, D still writing\n", acks, W)
	D.Slow = false
	fmt.Println("step 4: D done, copies:", B.Copies["cart"].Items, C.Copies["cart"].Items, D.Copies["cart"].Items)
	got := Get([]Version{C.Copies["cart"], D.Copies["cart"]})
	fmt.Println("step 5: read from C and D ->", got[0].Items)
	D.Up = false
	cart2 := Version{[]string{"headphones", "case"}, map[string]int{"B": 2}}
	acks, hint := Put(ring, "cart", cart2)
	fmt.Printf("step 6: D down: wrote B, C, E; %d answers; E holds a hint for %s: %v\n", acks, hint, strings.Join(E.Copies["cart"].Items, ", "))

	fmt.Println("split tab")
	B, C, D, E = mk("B"), mk("C"), mk("D"), mk("E")
	ring = []*Node{B, C, D, E}
	book := Version{[]string{"book"}, map[string]int{"B": 1}}
	Put(ring, "cart", book)
	fmt.Println("step 1: cart", B.Copies["cart"].Items, "clock", book.Clock)
	D.Up, E.Up = false, false
	pen := Version{[]string{"book", "pen"}, map[string]int{"B": 2}}
	acks, _ = Put(ring, "cart", pen)
	fmt.Printf("step 2-3: split; phone through B: %d answers >= W, cart %v, clock %v\n", acks, pen.Items, pen.Clock)
	D.Up, E.Up, B.Up, C.Up = true, true, false, false
	empty := Version{[]string{}, map[string]int{"B": 1, "D": 1}}
	acks, hint = Put(ring, "cart", empty)
	fmt.Printf("step 4: laptop through D: %d answers >= W, cart %v, clock %v, E holds a hint for %s\n", acks, empty.Items, empty.Clock, hint)
	B.Up, C.Up = true, true
	sib := Get([]Version{B.Copies["cart"], D.Copies["cart"]})
	fmt.Printf("step 5: split heals; read B and D: %d siblings\n", len(sib))
	fmt.Println("step 6: union merge ->", MergeUnion(sib[0], sib[1]), "(the book is back)")
}
