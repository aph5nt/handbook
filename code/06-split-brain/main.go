// Split brain: a cut network, two sides, and the "one copy of each entity" promise.
// Nodes are goroutines with channel mailboxes. The journal is shared storage both sides reach.
package main

import (
	"fmt"
	"sort"
	"sync"
)

type Event struct {
	Node     int
	Interval int
}

type Journal struct {
	mu     sync.Mutex
	events []Event
}

func (j *Journal) Append(e Event) { j.mu.Lock(); j.events = append(j.events, e); j.mu.Unlock() }

// Node holds its own idea of the cluster and runs the shards that map to it.
type Node struct {
	id      int
	members []int // the cluster as this node sees it
	mailbox chan func(*Node)
	device  int // device 42's interval if a copy runs here, 0 if not
}

func spawn(id int) *Node {
	n := &Node{id: id, members: []int{1, 2, 3}, mailbox: make(chan func(*Node))}
	go func() {
		for f := range n.mailbox {
			f(n)
		}
	}()
	return n
}

// do runs f inside the node's goroutine and waits for it.
func (n *Node) do(f func(*Node)) {
	done := make(chan struct{})
	n.mailbox <- func(n *Node) { f(n); close(done) }
	<-done
}

// home is where shard allocation puts device 42: node 2 if it is a member, else the lowest member.
func home(members []int) int {
	for _, m := range members {
		if m == 2 {
			return 2
		}
	}
	sort.Ints(members)
	return members[0]
}

// setInterval reaches device 42 through this node's side of the cluster.
func setInterval(via *Node, nodes map[int]*Node, j *Journal, s int) {
	var h int
	via.do(func(n *Node) { h = home(append([]int(nil), n.members...)) })
	nodes[h].do(func(n *Node) {
		if n.device == 0 {
			n.device = 60 // started from the journal
			fmt.Printf("  node %d: starts its own device 42 from the journal\n", n.id)
		}
		n.device = s
		j.Append(Event{n.id, s})
	})
}

func run(resolver string) {
	nodes := map[int]*Node{1: spawn(1), 2: spawn(2), 3: spawn(3)}
	j := &Journal{}
	nodes[2].do(func(n *Node) { n.device = 60 })
	sideA, sideB := []int{1, 2}, []int{3}

	fmt.Println("  the link to node 3 breaks: side A = nodes 1, 2, side B = node 3")
	for _, side := range [][]int{sideA, sideB} {
		size := len(side)
		for _, id := range side {
			s := append([]int(nil), side...)
			nodes[id].do(func(n *Node) {
				switch resolver {
				case "auto-down":
					n.members = s // down everyone unreachable
				case "keep-majority":
					if 2*size > 3 {
						n.members = s // majority: down the other side
					} else {
						n.members = nil // minority: down myself
						n.device = 0
					}
				}
				fmt.Printf("  node %d: %d of 3 reachable -> cluster %v\n", n.id, size, n.members)
			})
		}
	}

	setInterval(nodes[1], nodes, j, 30) // operator, through side A
	alive := false
	nodes[3].do(func(n *Node) { alive = n.members != nil })
	if alive {
		setInterval(nodes[3], nodes, j, 60) // device report, through side B
	} else {
		fmt.Println("  node 3: downed itself, the device report is refused and retried on side A later")
	}

	writers := map[int]bool{}
	for _, e := range j.events {
		writers[e.Node] = true
		fmt.Printf("  journal: node %d wrote interval %d s\n", e.Node, e.Interval)
	}
	fmt.Printf("  writers of device 42: %d\n", len(writers))
}

func main() {
	fmt.Println("== no resolver, auto-down on both sides")
	run("auto-down")
	fmt.Println("\n== split brain resolver: keep majority, after 20 s stable")
	run("keep-majority")
	fmt.Println("\n== the network heals")
	fresh := spawn(4) // node 3 restarts its actor system: a new identity
	fresh.do(func(n *Node) { n.members = []int{1, 2, 4} })
	fmt.Println("  node 3 rejoins as new member 4, holding no shard until rebalancing gives it some")
}
