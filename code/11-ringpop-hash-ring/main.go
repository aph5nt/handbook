// Command ringpop replays the Ringpop lesson: a consistent-hash ring, the
// clockwise owner, forwarding, gossip views, a join that moves few keys, and
// a network split with two owners. Ring positions are fixed so the run
// matches the picture; real Ringpop hashes the address to many virtual points.
package main

import (
	"fmt"
	"sort"
)

const trip42 = 135 // hash("trip 42") on the ring

var pos = map[string]int{"A": 0, "B": 90, "C": 180, "D": 270, "E": 160}

type Node struct {
	Name string
	View map[string]bool // who this node believes is alive
	Trip string          // its copy of trip 42
}

func Owner(key int, view map[string]bool) string {
	var names []string
	for n := range view {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool { return pos[names[i]] < pos[names[j]] })
	for _, n := range names {
		if pos[n] >= key {
			return n
		}
	}
	return names[0]
}

func (n *Node) Request(key int) string {
	o := Owner(key, n.View)
	if o == n.Name {
		return "handled at " + n.Name
	}
	return n.Name + " forwards to " + o
}

// Gossip marks every peer this node cannot reach as dead in its view.
func (n *Node) Gossip(reach func(a, b string) bool) {
	for p := range n.View {
		if !reach(n.Name, p) {
			delete(n.View, p)
		}
	}
}

// Moved counts keys 0..359 that change owner when a node joins.
func Moved(before, after map[string]bool) int {
	moved := 0
	for k := 0; k < 360; k++ {
		if Owner(k, before) != Owner(k, after) {
			moved++
		}
	}
	return moved
}

// Heal keeps only the true owner's copy; the other side's update is lost.
func Heal(all map[string]bool, nodes map[string]*Node) string { return nodes[Owner(trip42, all)].Trip }

func view(names ...string) map[string]bool {
	v := map[string]bool{}
	for _, n := range names {
		v[n] = true
	}
	return v
}

func main() {
	all := view("A", "B", "C", "D")
	fmt.Println("ring tab")
	fmt.Println("step 1: positions A 0, B 90, C 180, D 270; trip 42 at", trip42)
	fmt.Println("step 3: owner(trip 42) =", Owner(trip42, all))
	a := &Node{Name: "A", View: view("A", "B", "C", "D")}
	fmt.Println("step 4:", a.Request(trip42))
	withE := view("A", "B", "C", "D", "E")
	fmt.Printf("step 6: E joins at 160: owner %s, keys moved %d of 360\n", Owner(trip42, withE), Moved(all, withE))
	modMoved := 0
	for k := 0; k < 360; k++ {
		if k%4 != k%5 {
			modMoved++
		}
	}
	fmt.Printf("  plain hash %% N from 4 to 5 nodes would move %d of 360\n", modMoved)

	fmt.Println("split tab")
	fmt.Println("step 1: all agree, owner", Owner(trip42, all))
	sideOf := map[string]int{"A": 1, "B": 1, "C": 2, "D": 2}
	reach := func(x, y string) bool { return sideOf[x] == sideOf[y] }
	nodes := map[string]*Node{}
	for n := range all {
		nodes[n] = &Node{Name: n, View: view("A", "B", "C", "D")}
		nodes[n].Gossip(reach)
	}
	fmt.Println("step 2-3: A's view owner", Owner(trip42, nodes["A"].View), "; C's view owner", Owner(trip42, nodes["C"].View))
	nodes["A"].Trip = "driver arrived"
	nodes["C"].Trip = "rider cancelled"
	fmt.Printf("step 4: A holds %q, C holds %q\n", nodes["A"].Trip, nodes["C"].Trip)
	fmt.Printf("step 5: healed, owner %s keeps %q; %q is lost\n", Owner(trip42, all), Heal(all, nodes), nodes["A"].Trip)
}
