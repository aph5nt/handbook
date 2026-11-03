// Published language: a versioned table dictionary that names every column,
// its unit and its meaning. Readers learn the dictionary, not the indexer code.
package main

import (
	"fmt"
	"math/big"
)

// Column is one entry of the published language.
type Column struct {
	Name     string
	Unit     string // "" means the unit was never written down
	Decimals int    // divide the raw integer by 10^Decimals
}

// Dictionary is the contract, separate from the indexer's own structs.
type Dictionary struct {
	Version int
	Columns map[string]Column
}

// Read is what any reader does: look the column up, apply its unit.
func Read(d Dictionary, col, raw string) string {
	c := d.Columns[col]
	v, _ := new(big.Float).SetString(raw)
	if c.Unit == "" { // half a language: the reader guesses whole units
		return v.Text('f', 0) + " ETH (guessed)"
	}
	div := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(c.Decimals)), nil))
	return new(big.Float).Quo(v, div).Text('f', 1) + " " + c.Unit
}

// AddColumn grows the language in a new version. Old columns stay as they are.
func AddColumn(d Dictionary, c Column) Dictionary {
	cols := map[string]Column{}
	for k, v := range d.Columns {
		cols[k] = v
	}
	cols[c.Name] = c
	return Dictionary{Version: d.Version + 1, Columns: cols}
}

func main() {
	v1 := Dictionary{Version: 1, Columns: map[string]Column{
		"amount": {Name: "amount", Unit: "ETH", Decimals: 18},
	}}
	raw := "2500000000000000000"
	fmt.Println("v1 reader:", Read(v1, "amount", raw))
	v2 := AddColumn(v1, Column{Name: "fee", Unit: "ETH", Decimals: 18})
	fmt.Println("v2 has", len(v2.Columns), "columns; v1 reader still reads:", Read(v2, "amount", raw))
	noUnit := Dictionary{Version: 2, Columns: map[string]Column{"amount": {Name: "amount"}}}
	fmt.Println("no unit written:", Read(noUnit, "amount", raw))
	fmt.Println("unit back:", Read(v2, "amount", raw))
}
