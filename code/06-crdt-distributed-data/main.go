// CRDTs: a G-Counter and an OR-Set replicated by gossip between node goroutines.
// Merge takes the slot maximum or the set union, so order and repeats do not matter.
// A CRDT never says no, so a rule across nodes needs one owner instead.
package main

import (
	"fmt"
	"sort"
	"strings"
)

// GCounter has one slot per node. A node writes only its own slot.
type GCounter [3]int

func (c GCounter) Merge(o GCounter) GCounter {
	for i := range c {
		if o[i] > c[i] {
			c[i] = o[i] // the larger number wins, slot by slot
		}
	}
	return c
}

func (c GCounter) Total() int { return c[0] + c[1] + c[2] }

// ORSet stores each add under a unique tag. Merge is the union of the tags.
type ORSet map[string]string

func (s ORSet) Merge(o ORSet) ORSet {
	out := ORSet{}
	for k, v := range s {
		out[k] = v
	}
	for k, v := range o {
		out[k] = v
	}
	return out
}

func (s ORSet) String() string {
	vals := []string{}
	for _, v := range s {
		vals = append(vals, v)
	}
	sort.Strings(vals)
	return "{ " + strings.Join(vals, ", ") + " }"
}

// Node is a goroutine: it owns its copies and takes one message at a time.
type Node struct {
	id      int
	counter GCounter
	set     ORSet
	mailbox chan func(*Node)
}

func spawn(id, online int) *Node {
	n := &Node{id: id, set: ORSet{}, mailbox: make(chan func(*Node))}
	n.counter[id-1] = online
	go func() {
		for f := range n.mailbox {
			f(n)
		}
	}()
	return n
}

func (n *Node) do(f func(*Node)) {
	done := make(chan struct{})
	n.mailbox <- func(n *Node) { f(n); close(done) }
	<-done
}

func (n *Node) copyCounter() (c GCounter) {
	n.do(func(n *Node) { c = n.counter })
	return
}

// gossip sends a copy of the counter. The receiver merges it.
func gossip(c GCounter, to *Node) {
	to.do(func(n *Node) {
		n.counter = n.counter.Merge(c)
		fmt.Printf("  node %d: slots %v total %d\n", n.id, n.counter, n.counter.Total())
	})
}

func main() {
	fmt.Println("== G-Counter: online devices")
	n1, n2, n3 := spawn(1, 4000), spawn(2, 3500), spawn(3, 2500)
	gossip(n1.copyCounter(), n2)
	gossip(n2.copyCounter(), n3)
	gossip(n3.copyCounter(), n1)
	gossip(n3.copyCounter(), n2) // the next round

	old := n1.copyCounter()
	n1.do(func(n *Node) { n.counter[0] += 10 }) // a local write, no waiting
	c := n1.copyCounter()
	fmt.Printf("  node 1 local write: total %d, the others still %d\n", c.Total(), n2.copyCounter().Total())
	fmt.Println("  old copy reaches node 2:")
	gossip(old, n2)
	gossip(old, n2)
	fmt.Println("  new copy, twice, then on to node 3:")
	gossip(c, n2)
	gossip(c, n2)
	gossip(n2.copyCounter(), n3)

	fmt.Println("\n== OR-Set: a rule across nodes, split by a partition")
	book := func(n *Node, tag, job string) {
		n.do(func(n *Node) {
			if len(n.set) > 0 { // the rule, checked against this node's copy only
				fmt.Printf("  node %d: refuses %s\n", n.id, job)
				return
			}
			n.set[tag] = job
			fmt.Printf("  node %d: set was empty, accepts %s\n", n.id, job)
		})
	}
	book(n1, "n1-1", "device 42 at 10:00")
	book(n2, "n2-1", "device 17 at 10:00")
	fmt.Println("  the network heals, the sets merge:")
	var s1, s2 ORSet
	n1.do(func(n *Node) { s1 = n.set })
	n2.do(func(n *Node) { s2 = n.set })
	merged := s1.Merge(s2)
	fmt.Printf("  every node: %v -> %d bookings at 10:00, the rule is broken\n", merged, len(merged))

	fmt.Println("\n== the fix: one owner for the rule")
	owner := make(chan func())
	go func() {
		for f := range owner {
			f()
		}
	}()
	jobs := ORSet{}
	ask := func(job string) string {
		r := make(chan string)
		owner <- func() {
			if len(jobs) > 0 {
				r <- "refused"
				return
			}
			jobs[job] = job
			r <- "accepted"
		}
		return <-r
	}
	fmt.Println("  technician Anna: device 42 at 10:00:", ask("device 42 at 10:00"))
	fmt.Println("  technician Anna: device 17 at 10:00:", ask("device 17 at 10:00"))
	fmt.Println("  keep as CRDTs: the devices-online counter, ban lists, feature flags")
}
