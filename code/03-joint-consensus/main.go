// Joint consensus: growing a Raft cluster from nodes 1-2-3 to 1-to-5.
// It replays the lesson: a hand-edited member list lets two groups each
// win term 5, the joint configuration stops it, and one-node-at-a-time
// changes are safe because their majorities always overlap.
package main

import "fmt"

// Config is a member list. When New is set, the joint rule applies.
type Config struct {
	Old []int
	New []int // empty when not in a joint configuration
}

// IsMajority: more than half of members are among the votes.
func IsMajority(votes map[int]bool, members []int) bool {
	n := 0
	for _, m := range members {
		if votes[m] {
			n++
		}
	}
	return n > len(members)/2
}

// CanWin decides if a candidate with these votes wins under config c.
func CanWin(votes map[int]bool, c Config) bool {
	if len(c.New) == 0 {
		return IsMajority(votes, c.Old) // one list: one majority
	}
	// joint configuration: a majority of the old AND of the new list
	return IsMajority(votes, c.Old) && IsMajority(votes, c.New)
}

// Overlap reports whether every majority of a and every majority of b
// share at least one node. It tries every subset, for small clusters.
func Overlap(a, b []int) bool {
	all := map[int]bool{}
	for _, x := range append(append([]int{}, a...), b...) {
		all[x] = true
	}
	nodes := []int{}
	for x := range all {
		nodes = append(nodes, x)
	}
	for m1 := 0; m1 < 1<<len(nodes); m1++ {
		for m2 := 0; m2 < 1<<len(nodes); m2++ {
			s1, s2 := set(nodes, m1), set(nodes, m2)
			if IsMajority(s1, a) && IsMajority(s2, b) && !shares(s1, s2) {
				return false
			}
		}
	}
	return true
}

func set(nodes []int, mask int) map[int]bool {
	s := map[int]bool{}
	for i, n := range nodes {
		if mask&(1<<i) != 0 {
			s[n] = true
		}
	}
	return s
}

func shares(a, b map[int]bool) bool {
	for x := range a {
		if b[x] {
			return true
		}
	}
	return false
}

func main() {
	old, nw := []int{1, 2, 3}, []int{1, 2, 3, 4, 5}
	left := map[int]bool{1: true, 2: true}           // node 1 plus node 2's vote
	right := map[int]bool{3: true, 4: true, 5: true} // node 3 plus votes of 4 and 5

	fmt.Println("== Unsafe: node 1 uses the old list, node 3 was edited to the new list")
	fmt.Printf("left  {1,2}   under old list: wins=%v (2 of 3)\n", CanWin(left, Config{Old: old}))
	fmt.Printf("right {3,4,5} under new list: wins=%v (3 of 5)\n", CanWin(right, Config{Old: nw}))
	fmt.Println("two leaders in term 5")

	fmt.Println("== Joint configuration C-old,new on every node")
	j := Config{Old: old, New: nw}
	fmt.Printf("left  {1,2}:   old 2 of 3, new 2 of 5: wins=%v\n", CanWin(left, j))
	fmt.Printf("right {3,4,5}: old 1 of 3, new 3 of 5: wins=%v\n", CanWin(right, j))
	all := map[int]bool{1: true, 2: true, 3: true, 4: true, 5: true}
	fmt.Printf("split healed, all 5 vote: wins=%v, then C-new is written\n", CanWin(all, j))

	fmt.Println("== One node at a time")
	fmt.Printf("majorities of 1-2-3 and 1-2-3-4 always overlap: %v\n", Overlap(old, []int{1, 2, 3, 4}))
	fmt.Printf("majorities of 1-2-3 and 1-to-5 always overlap: %v\n", Overlap(old, nw))
}
