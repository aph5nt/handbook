// Data contracts: the producer's test checks its output against the
// written contract, so a unit change in amount_in fails before merge.
package main

import (
	"fmt"
	"math/big"
)

type Column struct {
	Name, Type, Unit string
	Deprecated       bool
}

type Contract struct {
	Version string
	Columns []Column
	Readers map[string][]string // column -> teams that read it
}

// Check compares the producer's new output with the contract. Removing a
// column, or changing its type or unit, is breaking. Adding one is not.
func (c Contract) Check(out []Column) []string {
	have := map[string]Column{}
	for _, o := range out {
		have[o.Name] = o
	}
	var errs []string
	for _, col := range c.Columns {
		o, ok := have[col.Name]
		switch {
		case !ok:
			errs = append(errs, col.Name+": removed")
		case o.Type != col.Type:
			errs = append(errs, col.Name+": type "+col.Type+" -> "+o.Type)
		case o.Unit != col.Unit:
			errs = append(errs, col.Name+": unit "+col.Unit+" -> "+o.Unit)
		}
	}
	return errs
}

// StillReading lists the teams that read a deprecated column.
func (c Contract) StillReading(col string) []string { return c.Readers[col] }

func main() {
	v1 := Contract{Version: "v1", Columns: []Column{
		{"block_height", "bigint", "", false}, {"log_index", "int", "", false},
		{"trader", "address", "", false}, {"amount_in", "decimal", "token units", false}},
		Readers: map[string][]string{"amount_in": {"graph sync", "daily volume report", "pricing join"}}}

	// the producer changes the unit, same name, same type
	out := []Column{v1.Columns[0], v1.Columns[1], v1.Columns[2], {"amount_in", "decimal", "raw units", false}}
	if errs := v1.Check(out); len(errs) > 0 {
		fmt.Println("pull request check: FAIL:", errs)
	}

	// expand: add amount_in_raw beside amount_in, deprecate the old column
	out = append(v1.Columns[:4:4], Column{"amount_in_raw", "decimal", "raw units", false})
	fmt.Println("v1.1 additive change, check errors:", len(v1.Check(out)))
	v1.Readers["amount_in"] = []string{"graph sync", "daily volume report"} // pricing join moved
	fmt.Println("still reading deprecated amount_in:", v1.StillReading("amount_in"))
	v1.Readers["amount_in"] = nil
	fmt.Println("readers left:", len(v1.StillReading("amount_in")), "-> drop amount_in, publish contract v2")

	// without a contract: the raw value reaches the readers as if it were tokens
	raw := new(big.Int).Mul(big.NewInt(125), new(big.Int).Exp(big.NewInt(10), big.NewInt(17), nil))
	fmt.Printf("no contract: 12.5 tokens shown as %s\n", raw.String()) // 12500000000000000000
	// what every reader computes, assuming token units
	type swapRow struct{ AmountIn float64 }
	volume := 0.0
	for _, row := range []swapRow{{AmountIn: 12.5e18}} {
		volume += row.AmountIn // now raw units: 10^18 times too big
	}
	fmt.Printf("daily volume report reads %.3g tokens for one 12.5 token swap\n", volume)
}
