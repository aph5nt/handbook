// Separate ways: billing checks wallet addresses itself instead of asking the
// graph, because the coupling costs more than twenty lines of code.
package main

import (
	"fmt"
	"strings"
)

// The link decision, made once for billing -> graph:
// gain: about twenty lines (validWallet below).
// cost: shared releases and shared outages.
const linkToGraph = false // gain < cost: separate ways

// Graph is the other context. During a rebuild from block 1 it answers nothing.
type Graph struct{ Rebuilding bool }

// IsWallet is the graph's answer, if billing chose to link.
func (g Graph) IsWallet(addr string) (bool, error) {
	if g.Rebuilding {
		return false, fmt.Errorf("graph rebuilding")
	}
	return validWallet(addr), nil
}

// validWallet is billing's own small check: 0x plus 40 hex characters.
func validWallet(addr string) bool {
	if len(addr) != 42 || !strings.HasPrefix(addr, "0x") {
		return false
	}
	for _, c := range addr[2:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

// TopUp is billing. linked decides whether it asks the graph or itself.
func TopUp(g Graph, addr string, linked bool) string {
	ok := validWallet(addr)
	if linked { // the coupling: billing now fails when the graph fails
		var err error
		if ok, err = g.IsWallet(addr); err != nil {
			return "top-up fails: " + err.Error()
		}
	}
	if !ok {
		return "top-up refused: bad address"
	}
	return "top-up accepted"
}

func main() {
	addr := "0x18a4c2d3e5f60718293a4b5c6d7e8f9012cb2400"
	up := Graph{}
	down := Graph{Rebuilding: true}
	fmt.Println("graph up, separate ways:", TopUp(up, addr, linkToGraph))
	fmt.Println("graph rebuilding, linked:", TopUp(down, addr, true))
	fmt.Println("graph rebuilding, separate ways:", TopUp(down, addr, linkToGraph))
	fmt.Println("bad address, separate ways:", TopUp(up, "0x12", false))
}
