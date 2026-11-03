// Shared kernel: a tiny model that two bounded contexts own together.
// A change to a kernel type needs a yes from every owner and green tests on every side.
package main

import (
	"fmt"
	"sort"
)

// Owner says which teams own a type. The kernel's types have two owners.
type Owner map[string][]string

var owners = Owner{
	"Receipt": {"indexer"}, "Decoder": {"indexer"},
	"Link": {"graph-sync"}, "Label": {"graph-sync"}, "Walk": {"graph-sync"},
}

var kernel = map[string]bool{}

// Share puts a type into the kernel: from now on both teams own it.
func Share(t string) {
	kernel[t] = true
	owners[t] = []string{"indexer", "graph-sync"}
}

// Unshare moves a type back into the one context that uses it.
func Unshare(t, team string) {
	delete(kernel, t)
	owners[t] = []string{team}
}

// Merge decides whether a change to type t can merge.
func Merge(t string, approved map[string]bool, testsPass map[string]bool) (bool, string) {
	need := owners[t] // every owner of the type must say yes
	for _, team := range need {
		if !approved[team] {
			return false, "waiting for " + team + " approval"
		}
		if !testsPass[team] {
			return false, team + " tests are red"
		}
	}
	return true, fmt.Sprintf("released for %v", need) // all owners rebuild
}

func kernelTypes() []string {
	var k []string
	for t := range kernel {
		k = append(k, t)
	}
	sort.Strings(k)
	return k
}

func main() {
	Share("Address")
	Share("BlockRange")
	fmt.Println("kernel:", kernelTypes())
	yes := map[string]bool{"indexer": true, "graph-sync": true}
	onlySync := map[string]bool{"graph-sync": true}
	ok, why := Merge("Address", onlySync, yes)
	fmt.Println("Address.Checksum, sync says yes:", ok, why)
	ok, why = Merge("Address", yes, map[string]bool{"indexer": true})
	fmt.Println("Address.Checksum, sync tests red:", ok, why)
	ok, why = Merge("Address", yes, yes)
	fmt.Println("Address.Checksum, both yes:", ok, why)
	// the failure: the kernel grows
	Share("RiskScore")
	Share("Labels")
	fmt.Println("kernel grew:", kernelTypes())
	ok, why = Merge("RiskScore", onlySync, yes)
	fmt.Println("risk rule change:", ok, why)
	// the fix: move them back to the context that owns them
	Unshare("RiskScore", "graph-sync")
	Unshare("Labels", "graph-sync")
	fmt.Println("kernel tiny again:", kernelTypes())
	ok, why = Merge("RiskScore", onlySync, yes)
	fmt.Println("risk rule change:", ok, why)
}
