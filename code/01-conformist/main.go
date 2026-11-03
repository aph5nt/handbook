// Conformist: the indexer uses the node's JSON-RPC names as is. Pricing did
// the same with a vendor that renames fields, until an adapter stopped it.
package main

import "fmt"

// Row is a decoded JSON object, keyed by the upstream's own names.
type Row map[string]float64

// places that read the upstream directly, in the upstream's words
var pricingPlaces = []string{"import", "daily close", "report", "alert", "graph value"}

// conformRead is a conformist read: it uses the upstream name directly.
func conformRead(r Row, name string) (float64, bool) {
	v, ok := r[name]
	return v, ok
}

// Adapter translates the vendor's row into our own word, usd.
// When the vendor renames px to price, only this function changes.
func Adapter(r Row) Row {
	if v, ok := r["price"]; ok {
		return Row{"usd": v}
	}
	return Row{"usd": r["px"]}
}

// RunPricing reads one row in every place. word is the name each place uses.
func RunPricing(r Row, word string) int {
	broken := 0
	for _, place := range pricingPlaces {
		if _, ok := conformRead(r, word); !ok {
			fmt.Println("  ", place+":", word, "missing")
			broken++
		}
	}
	return broken
}

func main() {
	block := Row{"blockNumber": 9506, "gasUsed": 21000}
	upgraded := Row{"blockNumber": 9507, "gasUsed": 21000} // new node release, same names
	n, _ := conformRead(block, "blockNumber")
	m, _ := conformRead(upgraded, "blockNumber")
	fmt.Println("indexer reads blockNumber:", n, "then after the node upgrade:", m)
	fmt.Println("vendor sends px:", RunPricing(Row{"px": 0.98}, "px"), "places broken")
	renamed := Row{"price": 0.98}
	fmt.Println("vendor renames px to price:")
	fmt.Println(RunPricing(renamed, "px"), "places broken")
	fmt.Println("with the adapter:", RunPricing(Adapter(renamed), "usd"), "places broken")
}
