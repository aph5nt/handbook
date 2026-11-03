// Ubiquitous language: one word per idea, with one rule, used in talk, code,
// tables and tests. Four names for one idea hid four different rules.
package main

import (
	"fmt"
	"strings"
)

// Addr is what the walk knows about one address.
type Addr struct {
	Name       string
	HotWallet  bool // on the team's hot-wallet list
	Depositors int  // how many customers deposit here
}

// Four names, four rules: each team's code is right by its own word.
func isCexHub(a Addr) bool   { return a.HotWallet }          // developer
func isVenue(a Addr) bool    { return a.Depositors >= 1000 } // warehouse table
func isExchange(a Addr) bool { return a.Depositors >= 1000 } // the agreed word

// Walk follows money and stops where stop says the trail breaks.
func Walk(path []Addr, stop func(Addr) bool) []string {
	var flagged []string
	for _, a := range path {
		if stop(a) {
			return flagged // money held for many customers: stop here
		}
		flagged = append(flagged, a.Name)
	}
	return flagged
}

// Glossary is the agreed word list. Review refuses any other name.
var Glossary = map[string]string{
	"Exchange": "holds money for many customers; the trace stops there",
	"Bridge":   "moves money between chains",
}
var banned = []string{"CexHub", "venue", "hub_detected"}

// Review is the habit that keeps one word per idea.
func Review(code string) error {
	for _, b := range banned {
		if strings.Contains(code, b) {
			return fmt.Errorf("refused: %s is not in the glossary, say Exchange", b)
		}
	}
	return nil
}

func main() {
	newEx := Addr{Name: "new exchange", Depositors: 4200}
	path := []Addr{{Name: "scammer"}, {Name: "mule"}, newEx, {Name: "customer 1"}, {Name: "customer 2"}}
	fmt.Println("venue says exchange:", isVenue(newEx), "- CexHub says:", isCexHub(newEx))
	fmt.Println("walk with CexHub flags:", Walk(path, isCexHub))
	fmt.Println("walk with Exchange flags:", Walk(path, isExchange))
	fmt.Println("glossary words:", len(Glossary))
	fmt.Println("review of a pull request with CexHub:", Review("type CexHub struct{}"))
	fmt.Println("review of exchange_is_found:", Review("func exchange_is_found()"))
}
