// Replays the distributed monolith lesson: four checkout services, the three
// symptoms (shared database, shared models library, synchronous chain), the
// Tuesday test, and the fix.
package main

import (
	"fmt"
	"sort"
)

type System struct {
	tables map[string][]string // table -> services that read or write it
	libs   map[string][]string // shared domain library -> services importing it
	calls  map[string][]string // service -> services it calls during a request
	up     float64             // availability of each service
}

var services = []string{"Checkout", "Customers", "Pricing", "Inventory"}

// coupledTo lists every other service s cannot ship without.
func (sys *System) coupledTo(s string) []string {
	others := map[string]bool{}
	for _, users := range sys.tables { // shared database
		addOthers(others, users, s)
	}
	for _, users := range sys.libs { // shared models library
		addOthers(others, users, s)
	}
	for _, callee := range sys.calls[s] { // must be up and compatible together
		others[callee] = true
	}
	var out []string
	for o := range others {
		out = append(out, o)
	}
	sort.Strings(out)
	return out
}

// canShipAlone is the Tuesday test.
func (sys *System) canShipAlone(s string) bool { return len(sys.coupledTo(s)) == 0 }

// pageAvailability multiplies the availability of every hop the page waits on.
func (sys *System) pageAvailability(start string) (float64, int) {
	a, hops, cur := sys.up, 1, start
	for len(sys.calls[cur]) > 0 {
		cur = sys.calls[cur][0]
		a *= sys.up
		hops++
	}
	return a, hops
}

func (sys *System) fix() {
	owner := map[string]string{"orders": "Checkout", "customers": "Customers", "prices": "Pricing", "stock": "Inventory"}
	for t := range sys.tables { // step 1: one owner per table
		sys.tables[t] = []string{owner[t]}
	}
	sys.calls = map[string][]string{} // step 2: events and local copies replace the chain
	sys.libs = map[string][]string{}  // step 3: versioned contracts replace the shared library
}

// addOthers marks every user of a shared thing, except s itself.
func addOthers(others map[string]bool, users []string, s string) {
	if !has(users, s) {
		return
	}
	for _, u := range users {
		if u != s {
			others[u] = true
		}
	}
}

func has(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func report(sys *System) {
	alone := 0
	for _, s := range services {
		c := sys.coupledTo(s)
		if len(c) == 0 {
			alone++
		}
		fmt.Printf("  %-9s coupled to %v\n", s, c)
	}
	fmt.Printf("  services that can ship alone: %d of %d\n", alone, len(services))
}

func main() {
	all := []string{"Checkout", "Customers", "Pricing", "Inventory"}
	sys := &System{
		tables: map[string][]string{"orders": all, "customers": all, "prices": all, "stock": all},
		libs:   map[string][]string{"common-models v13": all},
		calls:  map[string][]string{"Checkout": {"Customers"}, "Customers": {"Pricing"}, "Pricing": {"Inventory"}},
		up:     0.999,
	}
	a, hops := sys.pageAvailability("Checkout")
	fmt.Printf("symptom 1: one checkout page waits on %d services in a chain\n", hops)
	fmt.Printf("  page availability = 0.999^%d = %.4f (about %.1f percent)\n", hops, a, a*100)
	fmt.Printf("symptom 2: table prices is used by %v\n", sys.tables["prices"])
	fmt.Printf("symptom 3: common-models v13 is imported by %v\n", sys.libs["common-models v13"])
	fmt.Println("the Tuesday test, before the fix:")
	report(sys)

	sys.fix()
	a, hops = sys.pageAvailability("Checkout")
	fmt.Println("after the fix (one owner per table, events, versioned contracts):")
	fmt.Printf("  checkout page waits on %d service, availability %.3f\n", hops, a)
	report(sys)
}
