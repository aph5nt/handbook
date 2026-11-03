// FLP and Byzantine faults. Part 1: three nodes decide a lock holder while
// node 3 is silent. Waiting for every node never ends; a timeout lets the
// majority decide. Part 2: with one liar among three, a plain majority
// lets the two honest nodes decide opposite things. With four nodes,
// signed votes and a more-than-two-thirds rule, the honest nodes agree.
package main

import "fmt"

const never = -1 // node 3's reply never comes

// WaitAll waits for every reply before deciding. It returns the tick it
// decided, or never if it ran out of ticks still waiting.
func WaitAll(arrival []int, maxTicks int) int {
	for t := 0; t < maxTicks; t++ {
		got := 0
		for _, a := range arrival {
			if a != never && a <= t {
				got++
			}
		}
		if got == len(arrival) {
			return t
		}
	}
	return never // crashed or slow: without a clock it cannot tell
}

// WaitTimeout waits until the timeout, then decides with a majority.
func WaitTimeout(arrival []int, timeout int) int {
	for t := 0; ; t++ {
		got := 0
		for _, a := range arrival {
			if a != never && a <= t {
				got++
			}
		}
		if got == len(arrival) || (t >= timeout && got > len(arrival)/2) {
			return t // give up on the silent node, decide as a majority
		}
	}
}

// Vote is a message from one node to another.
type Vote struct {
	from, to int
	value    string
	signed   bool
}

// DecideMajority is the crash-only rule: 2 of 3 matching votes decide.
// It trusts every vote, so a liar can tip each honest node a different way.
func DecideMajority(me int, own string, inbox []Vote) string {
	count := map[string]int{own: 1}
	for _, v := range inbox {
		if v.to == me {
			count[v.value]++
			if count[v.value] >= 2 {
				return v.value
			}
		}
	}
	return "undecided"
}

// DecideBFT: votes are signed and relayed, so every node sees every vote.
// A node that signed two different values is a liar and is ignored.
// Decide only with matching votes from more than two thirds of n nodes.
func DecideBFT(all []Vote, n int) string {
	said := map[int]map[string]bool{}
	for _, v := range all {
		if said[v.from] == nil {
			said[v.from] = map[string]bool{}
		}
		said[v.from][v.value] = true
	}
	count := map[string]int{}
	for _, vals := range said {
		if len(vals) > 1 {
			continue // two different signed votes: a liar
		}
		for val := range vals {
			count[val]++
		}
	}
	for val, c := range count {
		if 3*c > 2*n {
			return val
		}
	}
	return "undecided"
}

func main() {
	fmt.Println("== FLP: node 3 is silent")
	arrival := []int{0, 1, never} // node 1 itself, node 2 at tick 1, node 3 never
	fmt.Printf("wait for all 3, up to 1000 ticks: decided at %d (never)\n", WaitAll(arrival, 1000))
	fmt.Printf("timeout 5 ticks, majority 2 of 3: decided at tick %d\n", WaitTimeout(arrival, 5))

	fmt.Println("== Three nodes, node 3 lies")
	lies := []Vote{{3, 1, "commit", false}, {3, 2, "abort", false}}
	fmt.Printf("node 1 (votes commit) decides: %s\n", DecideMajority(1, "commit", lies))
	fmt.Printf("node 2 (votes abort)  decides: %s\n", DecideMajority(2, "abort", lies))

	fmt.Println("== BFT: four nodes, signed votes, relayed to everyone")
	all := []Vote{
		{1, 0, "commit", true}, {2, 0, "commit", true}, {4, 0, "commit", true},
		{3, 1, "commit", true}, {3, 2, "abort", true}, // the liar, seen side by side
	}
	fmt.Printf("honest nodes decide: %s (3 of 4 > two thirds, liar ignored)\n", DecideBFT(all, 4))
}
