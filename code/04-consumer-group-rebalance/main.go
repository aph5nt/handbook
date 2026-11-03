// Consumer group rebalance: eager, a slow poll, and cooperative-sticky.
//
// A Go struct stands in for the group coordinator: it holds the plan
// (partition -> member), a generation number that grows on every
// rebalance, and the committed offset of each partition. Members keep
// their own in-memory position. Deterministic, no real Kafka.
package main

import "fmt"

type Group struct {
	gen       int
	owner     map[int]string
	committed map[int]int
}

// Commit is refused when it comes from an older generation.
func (g *Group) Commit(gen, p, off int) bool {
	if gen != g.gen {
		return false // the member was removed: its plan is stale
	}
	g.committed[p] = off
	return true
}

// Rebalance installs a new plan and returns the partitions that moved.
// Eager: every partition stops first. Cooperative: only moved ones stop.
func (g *Group) Rebalance(plan map[int]string, eager bool) (stopped, moved []int) {
	g.gen++
	for p := 0; p < 4; p++ {
		if plan[p] != g.owner[p] {
			moved = append(moved, p)
		}
		if eager || plan[p] != g.owner[p] {
			stopped = append(stopped, p)
		}
	}
	g.owner = plan
	return
}

const maxPollInterval = 300 // seconds, the Kafka default of 5 minutes
const sessionTimeout = 45   // seconds

func newGroup() *Group {
	return &Group{owner: map[int]string{0: "pod 1", 1: "pod 1", 2: "pod 2", 3: "pod 2"},
		committed: map[int]int{0: 800, 1: 500, 2: 300, 3: 300}}
}

func main() {
	fmt.Println("== Tab 1: eager ==")
	g := newGroup()
	position := 820 // pod 1 has processed P0 up to 820, last commit 800
	stopped, _ := g.Rebalance(map[int]string{0: "pod 3", 1: "pod 1", 2: "pod 2", 3: "pod 2"}, true)
	fmt.Printf("step 3: eager rebalance, generation %d, stopped partitions %v\n", g.gen, stopped)
	start := g.committed[0]
	dup := position - start // 820 - 800 = 20 read twice
	fmt.Printf("step 5: pod 3 starts P0 at %d, offsets %d..%d read twice: %d records\n", start, start, position-1, dup)

	fmt.Println("== Tab 2: slow poll ==")
	g = newGroup()
	gen := g.gen
	batchSeconds := 420 // very large blocks
	if batchSeconds > maxPollInterval {
		fmt.Printf("step 2: batch takes %d s > max.poll.interval.ms %d s: pod 2 is removed\n", batchSeconds, maxPollInterval)
		allToPod1 := map[int]string{0: "pod 1", 1: "pod 1", 2: "pod 1", 3: "pod 1"}
		g.Rebalance(allToPod1, true) // generation + 1
	}
	fmt.Printf("step 3: pod 1 starts P2 at %d. Pod 2's late commit accepted? %v\n", g.committed[2], g.Commit(gen, 2, 350))
	var cfg struct{ MaxPollRecords int }
	cfg.MaxPollRecords = 50 // about 50 s per poll, under 300 s
	fmt.Printf("step 5: fix: max.poll.records %d makes the batch %d s, under %d s\n", cfg.MaxPollRecords, cfg.MaxPollRecords, maxPollInterval)

	fmt.Println("== Tab 3: cooperative-sticky ==")
	g = newGroup()
	stopped, moved := g.Rebalance(map[int]string{0: "pod 3", 1: "pod 1", 2: "pod 2", 3: "pod 2"}, false)
	fmt.Printf("step 2: cooperative: moved %v, stopped %v, the rest keep flowing\n", moved, stopped)
	g.Commit(g.gen, 0, position) // commit on revoke
	fmt.Printf("step 4: commit on revoke, pod 3 starts P0 at %d, read twice: %d\n", g.committed[0], position-g.committed[0])
	restart := 20
	if restart < sessionTimeout {
		fmt.Printf("step 5: static member back in %d s < session timeout %d s: no rebalance, generation stays %d\n", restart, sessionTimeout, g.gen)
	}
}
