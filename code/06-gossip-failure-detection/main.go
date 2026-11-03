// Gossip membership and the phi accrual failure detector.
// Nodes are goroutines with channel mailboxes. Time is simulated in milliseconds.
package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Status of one member. A higher rank is newer: joining < up < down.
type Status int

const (
	Joining Status = iota
	Up
	Down
)

func (s Status) String() string { return [...]string{"joining", "up", "down"}[s] }

type View map[int]Status

// merge keeps the newest status of each member. Order of merges does not matter.
func merge(a, b View) View {
	out := View{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if cur, ok := out[k]; !ok || v > cur {
			out[k] = v
		}
	}
	return out
}

func show(v View) string {
	keys := []int{}
	for k := range v {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	parts := []string{}
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%d %v", k, v[k]))
	}
	return strings.Join(parts, ", ")
}

// node is a goroutine that merges every view it is gossiped.
type node struct {
	id      int
	mailbox chan View
	view    View
	reply   chan View
}

func spawn(id int, v View) *node {
	n := &node{id: id, mailbox: make(chan View), view: v, reply: make(chan View)}
	go func() {
		for in := range n.mailbox {
			n.view = merge(n.view, in)
			n.reply <- n.view
		}
	}()
	return n
}

func gossip(from, to *node) View { to.mailbox <- from.view; return <-to.reply }

// phi is Akka's phi accrual: how unlikely is a gap this long, given the gaps seen so far?
// The acceptable pause is added to the mean, as Akka does.
func phi(elapsed, mean, std, acceptable float64) float64 {
	y := (elapsed - (mean + acceptable)) / std
	e := math.Exp(-y * (1.5976 + 0.070566*y*y))
	if elapsed > mean+acceptable {
		return -math.Log10(e / (1 + e))
	}
	return -math.Log10(1 - 1/(1+e))
}

func main() {
	fmt.Println("== join by gossip")
	n1 := spawn(1, View{1: Up, 2: Up})
	n2 := spawn(2, View{1: Up, 2: Up})
	n3 := spawn(3, View{3: Joining})
	n1.view = merge(n1.view, View{3: Joining}) // node 3 contacts seed node 1
	fmt.Println("node 1 view:", show(n1.view))
	fmt.Println("round 1, node 1 -> node 2:", show(gossip(n1, n2)))
	fmt.Println("round 2, node 2 -> node 3:", show(gossip(n2, n3)))
	converged := show(n1.view) == show(n2.view) && show(n2.view) == show(n3.view)
	fmt.Println("converged:", converged)
	if converged {
		leader := 1 // the lowest address, not elected
		n1.view[3] = Up
		gossip(n1, n2)
		gossip(n2, n3)
		fmt.Printf("leader node %d moves node 3 to up: %s\n", leader, show(n3.view))
	}

	const threshold = 8.0
	const mean, std = 1000.0, 200.0 // heartbeats about once a second
	for _, tc := range []struct {
		name       string
		acceptable float64
		autoDown   float64 // 0: off
	}{
		{"garbage collection pause, pause limit 3 s, no auto-down", 3000, 0},
		{"the failure: pause limit 3 s, auto-down after 2 s unreachable", 3000, 2000},
		{"the fix: pause limit 10 s, above the worst pause of 8 s", 10000, 0},
	} {
		fmt.Println("\n==", tc.name)
		unreachableAt := -1.0
		downed := false
		for t := 1000.0; t <= 8000; t += 1000 { // node 3 is frozen for 8 s
			p := phi(t, mean, std, tc.acceptable)
			state := "reachable"
			if p > threshold {
				if unreachableAt < 0 {
					unreachableAt = t
				}
				state = "unreachable"
				if tc.autoDown > 0 && t-unreachableAt >= tc.autoDown {
					downed = true
				}
			}
			if downed {
				state = "DOWN: shards restart on nodes 1 and 2"
			}
			fmt.Printf("  %4.0f ms since last heartbeat: phi %6.2f -> %s\n", t, math.Max(0, math.Min(p, 99)), state)
			if downed {
				break
			}
		}
		if !downed {
			back := math.Max(0, phi(mean, mean, std, tc.acceptable)) // a returning heartbeat: the gap is 1 s again
			fmt.Printf("  8 s: pause ends, heartbeat returns, phi %.2f, node 3 reachable, nothing moved\n", back)
		} else {
			fmt.Println("  node 3 wakes up out of the cluster")
		}
	}
}
