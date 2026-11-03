// Split brain resolver: every node applies the same rule to the same membership,
// so the two sides agree without a single message across the cut.
package main

import "fmt"

// Rule answers one question for one side: do I keep running?
// side is the nodes this node can reach. all is the cluster, oldest first.
type Rule func(side, all []int) bool

func has(s []int, n int) bool {
	for _, x := range s {
		if x == n {
			return true
		}
	}
	return false
}

func lowest(s []int) int {
	m := s[0]
	for _, x := range s {
		if x < m {
			m = x
		}
	}
	return m
}

func keepMajority(side, all []int) bool {
	if 2*len(side) != len(all) {
		return 2*len(side) > len(all)
	}
	return has(side, lowest(all)) // an even split: the lowest address wins
}

func keepOldest(downIfAlone bool) Rule {
	return func(side, all []int) bool {
		oldest := all[0]
		if downIfAlone && len(side) == 1 && side[0] == oldest && len(all) > 2 {
			return false // the oldest, alone: down myself
		}
		if downIfAlone && len(all)-len(side) == 1 && !has(side, oldest) {
			return true // the other side is the oldest, alone: it downs itself
		}
		return has(side, oldest)
	}
}

func staticQuorum(size int) Rule {
	return func(side, all []int) bool { return len(side) >= size }
}

// split runs the rule on one goroutine per node and collects the decisions.
// A node hears nothing from across the cut: it sees only its own side.
func split(name string, rule Rule, a, b []int) {
	all := append(append([]int(nil), a...), b...)
	fmt.Printf("== %s, side A %v, side B %v\n", name, a, b)
	type decision struct {
		node int
		keep bool
	}
	out := make(chan decision)
	for _, side := range [][]int{a, b} {
		for _, n := range side {
			mailbox := make(chan []int, 1)
			mailbox <- side // the stable view, after stable-after = 20 s
			go func(n int) { out <- decision{n, rule(<-mailbox, all)} }(n)
		}
	}
	kept := map[int]bool{}
	for range all {
		d := <-out
		kept[d.node] = d.keep
	}
	sides := 0
	for _, side := range [][]int{a, b} {
		if len(side) > 0 && kept[side[0]] {
			sides++
		}
		fmt.Printf("  side %v (%d of %d): keep=%v\n", side, len(side), len(all), kept[side[0]])
	}
	survivors := 0
	for _, n := range all {
		if kept[n] {
			survivors++
		}
	}
	switch {
	case sides == 2:
		fmt.Println("  both sides live: every moved entity runs twice, two writers per journal")
	case sides == 0:
		fmt.Println("  both sides down: the whole cluster stopped")
	default:
		fmt.Printf("  10 shards on %d node(s): %d per node at most\n", survivors, (10+survivors-1)/survivors)
	}
}

func main() {
	split("keep majority, five nodes", keepMajority, []int{1, 2, 3}, []int{4, 5})
	split("keep majority, four nodes, a tie", keepMajority, []int{1, 2}, []int{3, 4})
	split("keep oldest, oldest alone", keepOldest(false), []int{1}, []int{2, 3, 4, 5})
	split("keep oldest with down-if-alone", keepOldest(true), []int{1}, []int{2, 3, 4, 5})
	split("static quorum 2, three nodes", staticQuorum(2), []int{1, 2}, []int{3})
	split("static quorum 2, cluster grew to five", staticQuorum(2), []int{1, 2, 3}, []int{4, 5})
	split("static quorum 3, five nodes", staticQuorum(3), []int{1, 2, 3}, []int{4, 5})
}
