// Customer-supplier: the warehouse (upstream, supplier) runs the graph sync's
// contract test (downstream, customer) in its own build.
package main

import (
	"fmt"
	"strings"
)

// Table is core_transfers: column name -> type.
type Table map[string]string

// contract is written by the customer: the columns it reads, with their types.
var contract = Table{"value": "LARGEINT", "from": "VARCHAR", "to": "VARCHAR", "fee": "LARGEINT"}

// ContractTest runs inside the supplier's build on every change.
func ContractTest(t Table, reads Table) []string {
	var missing []string
	for col, typ := range reads {
		if t[col] != typ {
			missing = append(missing, col)
		}
	}
	return missing
}

// Build is the warehouse pipeline. Without the contract test, only its own tests run.
func Build(t Table, reads Table, withContract bool) string {
	if withContract {
		if m := ContractTest(t, reads); len(m) > 0 {
			return "build red, merge blocked: customer reads " + strings.Join(m, ",")
		}
	}
	return "build green, merged"
}

func clone(t Table) Table {
	c := Table{}
	for k, v := range t {
		c[k] = v
	}
	return c
}

func main() {
	t := Table{"amount": "LARGEINT", "from": "VARCHAR", "to": "VARCHAR"}
	t["fee"] = "LARGEINT" // the customer asked; the supplier delivers
	reads := Table{"amount": "LARGEINT", "from": "VARCHAR", "to": "VARCHAR", "fee": "LARGEINT"}
	// one-step rename, no contract test: merges, breaks the reader at night
	oneStep := clone(t)
	delete(oneStep, "amount")
	oneStep["value"] = "LARGEINT"
	fmt.Println("rename in one step, no contract:", Build(oneStep, reads, false))
	fmt.Println("rename in one step, contract test:", Build(oneStep, reads, true))
	// add, move, drop
	t["value"] = "LARGEINT"
	fmt.Println("add value, keep amount:", Build(t, reads, true))
	reads = contract // the customer moves its reads to value
	delete(t, "amount")
	fmt.Println("reader moved, drop amount:", Build(t, reads, true))
}
