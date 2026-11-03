// Replays the coupling and cohesion lesson: the dex module asks the pricing
// module for a day price. It finds the five kinds of coupling between them,
// shows which ones an import graph can see, and checks a utils package for
// cohesion. Then it applies the fix.
package main

import (
	"fmt"
	"math"
	"strings"
)

// Coupling is one way dex depends on pricing.
type Coupling struct {
	kind    string // code, data, temporal, deploy, semantic
	what    string
	inCode  bool // does an import graph show it?
	written bool // is it written into the contract and tested?
}

var dexToPricing = []Coupling{
	{"code", "calls DayPrice(token, day)", true, true},
	{"data", "reads table pricing.day_prices", false, false},
	{"temporal", "nightly job runs after the day close", false, false},
	{"deploy", "a DayPrice change must ship with dex", false, false},
	{"semantic", "Quote.Amount is raw, scaled by decimals", false, false},
}

// has says whether a kind of coupling is still there.
func has(cs []Coupling, kind string) bool {
	for _, c := range cs {
		if c.kind == kind {
			return true
		}
	}
	return false
}

// renameBreaksDex: pricing renames a column of its own table. Only a data
// coupling breaks dex. No import changes, so no build fails.
func renameBreaksDex(cs []Coupling) bool { return has(cs, "data") }

// shipTogether: DayPrice gains a currency argument. Must both modules ship
// in one release? Not with a versioned contract: the old DayPrice stays.
func shipTogether(cs []Coupling) bool { return has(cs, "deploy") }

// nightly: dex prices the day's swaps at 02:00. pricing closes the day at
// closeAt. A swap read before the close gets no price, unless the run
// order is written down and dex waits for the close.
func nightly(swaps int, closeAt, dexAt string, waits bool) (unpriced int) {
	if closeAt > dexAt && !waits {
		return swaps
	}
	return 0
}

// Quote is what pricing sends: USD for Amount of the token. Both modules
// assume Amount is raw, scaled by the token's decimals.
type Quote struct{ Amount, USD float64 }

// swapValue is dex's maths: whole tokens swapped / whole tokens quoted x USD.
func swapValue(raw float64, decimals int, q Quote) float64 {
	scale := math.Pow10(decimals)
	return (raw / scale) / (q.Amount / scale) * q.USD
}

// reasons counts the different reasons a package changes for.
func reasons(funcs map[string]string) int {
	seen := map[string]bool{}
	for _, why := range funcs {
		seen[why] = true
	}
	return len(seen)
}

// fix: dex reads the price through DayPrice only, DayPrice gets a versioned
// contract, and the run order and the unit are written into the contract.
func fix(cs []Coupling) []Coupling {
	var out []Coupling
	for _, c := range cs {
		switch c.kind {
		case "data", "deploy":
			continue // the shared table and the lockstep release are gone
		case "temporal", "semantic":
			c.written = true // written down and tested
		}
		out = append(out, c)
	}
	return out
}

func main() {
	seen := 0
	for _, c := range dexToPricing {
		if c.inCode {
			seen++
		}
		fmt.Printf("%-9s %-40s in the import graph: %v\n", c.kind, c.what, c.inCode)
	}
	fmt.Printf("the import graph shows %d of %d kinds\n", seen, len(dexToPricing))

	fmt.Printf("data: pricing renames a column, dex breaks: %v, and no import changed, so the build still passes\n", renameBreaksDex(dexToPricing))
	fmt.Printf("temporal: pricing closes at 02:40, dex runs at 02:00: %d of 1200 swaps get no price\n",
		nightly(1200, "02:40", "02:00", false))
	fmt.Printf("deploy: DayPrice gains a currency argument: dex and pricing ship together: %v\n", shipTogether(dexToPricing))

	raw, dec := 1.5e18, 18                      // 1.5 tokens with 18 decimals
	good := swapValue(raw, dec, Quote{1e18, 2}) // one whole token, raw, for 2 USD
	bad := swapValue(raw, dec, Quote{1, 2})     // pricing now sends the human amount
	fmt.Printf("semantic: value %.2f USD becomes %.1e USD, wrong by a factor of %.0e\n", good, bad, bad/good)

	utils := map[string]string{"FormatDate": "dates", "Retry": "retries", "RenderHTML": "HTML", "FeeBps": "fees"}
	fmt.Printf("cohesion: utils has %d functions and %d reasons to change\n", len(utils), reasons(utils))

	left := fix(dexToPricing)
	var names []string
	for _, c := range left {
		names = append(names, fmt.Sprintf("%s (written: %v)", c.kind, c.written))
	}
	fmt.Printf("after the fix: %d kinds left: %s\n", len(left), strings.Join(names, ", "))
	fmt.Printf("  rename a column, dex breaks: %v\n", renameBreaksDex(left))
	fmt.Printf("  DayPrice gains a currency argument: dex and pricing ship together: %v\n", shipTogether(left))
	fmt.Printf("  pricing closes at 02:40, dex waits for the close: %d swaps get no price\n",
		nightly(1200, "02:40", "02:00", true))
	fmt.Printf("  utils split into %d packages with 1 reason to change each\n", reasons(utils))
}
