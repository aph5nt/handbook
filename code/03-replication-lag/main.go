// Replication: a Postgres-style leader with two followers, then a
// Cassandra-style leaderless key. It replays the lesson: lag as a log
// position, a failover to the wrong follower, last-writer-wins dropping a
// write, and version vectors keeping both writes for the app to merge.
package main

import (
	"fmt"
	"sort"
)

// Node holds the log positions it has applied.
type Node struct {
	name string
	pos  int
	dead bool
}

// Leader appends a write at the next log position.
func Append(leader *Node) int {
	leader.pos++
	return leader.pos
}

// Ship copies the leader's log to a follower, up to the position that has
// reached it. A slow follower gets a smaller upTo.
func Ship(leader, f *Node, upTo int) {
	if upTo > leader.pos {
		upTo = leader.pos
	}
	if f.pos < upTo {
		f.pos = upTo
	}
}

// Lag is the leader's position minus the follower's.
func Lag(leader, f *Node) int { return leader.pos - f.pos }

// PromoteCareless picks whichever follower answered first (here follower 2),
// whatever its log position.
func PromoteCareless(followers []*Node) *Node { return followers[len(followers)-1] }

// PromoteSafe picks the follower with the highest log position.
func PromoteSafe(followers []*Node) *Node {
	best := followers[0]
	for _, f := range followers {
		if f.pos > best.pos {
			best = f
		}
	}
	return best
}

// Version is one write to a leaderless key.
type Version struct {
	value string
	stamp string         // wall-clock stamp from the writer's clock
	vv    map[string]int // version vector: writer -> count
}

// LWW keeps the version with the larger clock stamp, and drops the other.
func LWW(a, b Version) Version {
	if b.stamp > a.stamp {
		return b
	}
	return a
}

// Descends reports whether vector a has seen everything vector b has.
func Descends(a, b map[string]int) bool {
	for k, n := range b {
		if a[k] < n {
			return false
		}
	}
	return true
}

// Merge keeps one version if it descends from the other, else both siblings.
func Merge(a, b Version) []Version {
	if Descends(a.vv, b.vv) {
		return []Version{a}
	}
	if Descends(b.vv, a.vv) {
		return []Version{b}
	}
	return []Version{a, b} // concurrent: keep both for the app
}

func main() {
	fmt.Println("== Leader-follower")
	L, F1, F2 := &Node{"leader", 4, false}, &Node{"follower 1", 4, false}, &Node{"follower 2", 4, false}
	fmt.Printf("start: positions %d %d %d\n", L.pos, F1.pos, F2.pos)
	p := Append(L)
	fmt.Printf("insert order 5: leader writes position %d\n", p)
	Ship(L, F1, 5)
	Ship(L, F2, 4) // one second behind
	fmt.Printf("ship: follower 1 at %d, follower 2 at %d, lag %d\n", F1.pos, F2.pos, Lag(L, F2))
	fmt.Printf("report on follower 2 counts %d orders, leader has %d\n", F2.pos, L.pos)
	p = Append(L)
	Ship(L, F1, p)
	fmt.Printf("order 6: leader %d, follower 1 %d, follower 2 %d, lag %d\n", L.pos, F1.pos, F2.pos, Lag(L, F2))
	L.dead = true
	followers := []*Node{F1, F2}
	bad := PromoteCareless(followers)
	fmt.Printf("leader dies, careless promote: %s at %d, lost %d confirmed orders\n", bad.name, bad.pos, L.pos-bad.pos)
	good := PromoteSafe(followers)
	fmt.Printf("fix, promote highest position: %s at %d, lost %d\n", good.name, good.pos, L.pos-good.pos)

	fmt.Println("== Leaderless")
	x := Version{"exchange", "10:00:05", map[string]int{"X": 1}} // X's clock runs 3 s fast
	y := Version{"mule", "10:00:03", map[string]int{"Y": 1}}     // written later in real time
	w := LWW(x, y)
	fmt.Printf("last-writer-wins: keeps %q (stamp %s), drops %q\n", w.value, w.stamp, y.value)
	sib := Merge(x, y)
	vals := []string{}
	for _, v := range sib {
		vals = append(vals, v.value)
	}
	sort.Strings(vals)
	fmt.Printf("version vectors {X:1} and {Y:1}: concurrent, keep %d siblings %v\n", len(sib), vals)
	m := Version{"mule", "10:00:04", map[string]int{"X": 1, "Y": 1}}
	fmt.Printf("app merges to %q with {X:1, Y:1}: replaces both: %v\n", m.value, len(Merge(m, x)) == 1 && len(Merge(m, y)) == 1)
}
