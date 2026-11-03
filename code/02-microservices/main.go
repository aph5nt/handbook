// Replays the microservices lesson. Tab 1: the indexer, the graph sync and
// the price oracle each own their data, ship alone, scale alone, and a trace
// names the slow hop. Tab 2: five services share one schema and call each
// other in a chain: the distributed monolith, and its fix.
package main

import (
	"fmt"
	"math"
)

type System struct {
	tables   map[string][]string // table -> services that use it
	calls    map[string]string   // service -> the service it calls and waits on
	work     map[string]int      // own work of each service, in ms
	replicas map[string]int
}

// releasesNeeded: a change to service s ships s plus every other user of its tables.
func (sys *System) releasesNeeded(s string) int {
	ship := map[string]bool{s: true}
	for _, users := range sys.tables {
		if contains(users, s) { // every user of a table s uses ships with s
			for _, u := range users {
				ship[u] = true
			}
		}
	}
	return len(ship)
}

// request walks the call chain. Each hop adds its own work and waits on the next.
func (sys *System) request(start string, timeoutMs int) (int, []string, bool) {
	total, waiting := 0, []string{}
	for cur := start; cur != ""; cur = sys.calls[cur] {
		total += sys.work[cur]
		if sys.calls[cur] != "" {
			waiting = append(waiting, cur) // holds its request open
		}
	}
	return total, waiting, total <= timeoutMs
}

// scale adds a copy of one service only. The others keep theirs.
func (sys *System) scale(s string) { sys.replicas[s]++ }

// fix moves each table under one owner. The others use its API or events.
func (sys *System) fix(owner map[string]string) {
	for t := range sys.tables {
		sys.tables[t] = []string{owner[t]}
	}
}

// Span is one hop in a trace.
type Span struct {
	name string
	ms   int
}

// slowestHop is what a trace gives you: the hop that took the time.
func slowestHop(trace []Span) Span {
	worst := trace[0]
	for _, s := range trace[1:] {
		if s.ms > worst.ms {
			worst = s
		}
	}
	return worst
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func main() {
	fmt.Println("tab 1: real microservices")
	real := &System{
		tables:   map[string][]string{"indexer rows": {"Indexer"}, "graph": {"Graph sync"}, "prices": {"Price oracle"}},
		replicas: map[string]int{"Indexer": 1, "Graph sync": 1, "Price oracle": 1},
	}
	shared := 0
	for _, users := range real.tables {
		if len(users) > 1 {
			shared++
		}
	}
	fmt.Printf("  shared tables: %d\n", shared)
	fmt.Printf("  graph sync ships v2: releases needed %d\n", real.releasesNeeded("Graph sync"))
	real.scale("Graph sync")
	fmt.Printf("  graph sync copies: %d, indexer copies: %d\n", real.replicas["Graph sync"], real.replicas["Indexer"])
	timeout := 2000
	trace := []Span{{"read indexer rows", 120}, {"price API", 2400}, {"write graph", 300}}
	sum := 0
	for _, s := range trace {
		sum += s.ms
	}
	w := slowestHop(trace)
	fmt.Printf("  graph sync page: %d ms, timeout %d ms, slowest hop by the trace: %s %d ms\n", sum, timeout, w.name, w.ms)

	fmt.Println("tab 2: distributed monolith")
	all := []string{"Service 1", "Service 2", "Service 3", "Service 4", "Service 5"}
	fake := &System{
		tables: map[string][]string{"orders": all, "accounts": all, "items": all},
		calls:  map[string]string{"Service 1": "Service 2", "Service 2": "Service 3", "Service 3": "Service 4", "Service 4": "Service 5"},
		work:   map[string]int{"Service 1": 20, "Service 2": 20, "Service 3": 20, "Service 4": 20, "Service 5": 20},
	}
	fmt.Printf("  owners per table: %d\n", len(fake.tables["orders"]))
	ms, waiting, ok := fake.request("Service 1", timeout)
	fmt.Printf("  one request: %d ms through %d services, ok=%v\n", ms, len(waiting)+1, ok)
	fmt.Printf("  chain availability: 0.999^5 = %.4f\n", math.Pow(0.999, 5))
	fake.work["Service 5"] = 3000
	ms, waiting, ok = fake.request("Service 1", timeout)
	fmt.Printf("  service 5 slow: %d ms, waiting %v, ok=%v (timeout %d ms)\n", ms, waiting, ok, timeout)
	fmt.Printf("  rename one column of orders: releases needed %d, in one window\n", fake.releasesNeeded("Service 3"))

	fake.fix(map[string]string{"orders": "Service 1", "accounts": "Service 2", "items": "Service 3"})
	fmt.Printf("  after the fix: owners per table 1, rename releases needed %d\n", fake.releasesNeeded("Service 1"))
}
