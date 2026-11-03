// Gossip: 16 nodes, node F dies, and the news "F is dead" spreads by each
// informed node telling one peer per round. It replays the lesson's rounds,
// the stale-view failure, and a SWIM indirect check.
package main

import (
	"fmt"
	"math"
)

// Entry is what a node believes about one other node.
type Entry struct {
	Heartbeat int  // the counter that node raises every second
	Dead      bool // marked dead once its counter stalled
}

// Node holds its own view of the whole cluster.
type Node struct {
	Name string
	View map[string]Entry
}

// Merge swaps views with a peer. For each node, both keep the newer entry:
// the higher heartbeat wins, and at the same heartbeat "dead" wins.
func Merge(a, b *Node) {
	for name := range union(a.View, b.View) {
		x, y := a.View[name], b.View[name]
		newer := x
		if y.Heartbeat > x.Heartbeat || (y.Heartbeat == x.Heartbeat && y.Dead) {
			newer = y
		}
		a.View[name], b.View[name] = newer, newer
	}
}

// CheckStalled marks a node dead when its heartbeat has not risen for
// timeout seconds.
func (n *Node) CheckStalled(name string, stalledFor, timeout int) {
	if stalledFor >= timeout {
		e := n.View[name]
		e.Dead = true
		n.View[name] = e
	}
}

// Route sends a request to the first node this view thinks is alive.
// A view can be stale, so a timed-out call is retried on the next node.
// Never use a view to elect a leader or grant a lock: views differ.
func Route(from *Node, order []string, reachable func(string) bool) (string, []string) {
	var failed []string
	for _, target := range order {
		if from.View[target].Dead {
			continue // my view says dead: skip
		}
		if reachable(target) {
			return target, failed
		}
		failed = append(failed, target) // timed out: my view was stale
	}
	return "", failed
}

// SwimProbe: a direct ping failed, so ask k helpers to ping the target.
// The target is only suspected when no helper reaches it either.
func SwimProbe(target string, helpers []string, canReach func(from, to string) bool) string {
	for _, h := range helpers {
		if canReach(h, target) {
			return "alive" // the link from me was bad, not the node
		}
	}
	return "suspected"
}

func union(a, b map[string]Entry) map[string]bool {
	u := map[string]bool{}
	for k := range a {
		u[k] = true
	}
	for k := range b {
		u[k] = true
	}
	return u
}

func main() {
	names := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P"}
	nodes := map[string]*Node{}
	for _, n := range names {
		nodes[n] = &Node{n, map[string]Entry{}}
		for _, m := range names {
			nodes[n].View[m] = Entry{Heartbeat: 7}
		}
	}
	knows := func() int {
		c := 0
		for _, n := range names {
			if n != "F" && nodes[n].View["F"].Dead {
				c++
			}
		}
		return c
	}
	fmt.Println("== The rumour spreads")
	nodes["A"].CheckStalled("F", 5, 5) // F's counter stalled 5 s, timeout 5 s
	fmt.Printf("round 1: A marks F dead, %d of 15 know\n", knows())
	rounds := [][][2]string{
		{{"A", "K"}},
		{{"A", "D"}, {"K", "N"}},
		{{"A", "H"}, {"K", "C"}, {"D", "O"}, {"N", "B"}},
		{{"A", "E"}, {"K", "G"}, {"D", "I"}, {"N", "J"}, {"H", "L"}, {"C", "M"}, {"O", "P"}},
	}
	for r, pairs := range rounds {
		for _, p := range pairs {
			Merge(nodes[p[0]], nodes[p[1]])
		}
		fmt.Printf("round %d: %d of 15 know\n", r+2, knows())
	}
	for _, n := range []int{16, 1000, 1000000} {
		fmt.Printf("%d nodes: about %.0f rounds\n", n, math.Ceil(math.Log2(float64(n))))
	}

	fmt.Println("== Stale views")
	stale := &Node{"M", map[string]Entry{"F": {Heartbeat: 7}, "G": {Heartbeat: 7}}}
	alive := func(t string) bool { return t != "F" }
	got, failed := Route(stale, []string{"F", "G"}, alive)
	fmt.Printf("M still thinks F is alive: call to %v times out, retried on %s\n", failed, got)
	link := func(from, to string) bool { return !(from == "A" && to == "G") }
	fmt.Printf("SWIM: A cannot reach G, asks B and C: G is %s\n", SwimProbe("G", []string{"B", "C"}, link))
}
