// Message id and causation id: three ids, one question each.
//
// Plain structs stand in for messages and a map for the consumer's inbox.
// Traces one chain of three events after the request (TopUpPaid, UnitsGranted,
// ReceiptSent), drops a duplicate, misses one with a new id, and finds an
// event cycle by walking the causation ids. Deterministic.
package main

import (
	"fmt"
	"sort"
	"strings"
)

type Msg struct {
	ID, Name, Correlation, Causation string
}

// child makes a new message caused by the one being handled.
func child(parent Msg, id, name string) Msg {
	return Msg{id, name, parent.Correlation, parent.ID}
}

type Inbox map[string]bool

// Seen is the first check of every handler.
func (in Inbox) Seen(id string) bool {
	if in[id] {
		return true
	}
	in[id] = true
	return false
}

// chain walks causation ids from a message back to the root.
func chain(all map[string]Msg, id string) []string {
	var out []string
	for id != "" {
		m := all[id]
		out = append([]string{m.Name + " " + m.ID}, out...)
		id = m.Causation
	}
	return out
}

// hasCycle reports whether an event type repeats on the way to the root.
func hasCycle(all map[string]Msg, id string) (string, bool) {
	seen := map[string]bool{}
	for id != "" {
		m := all[id]
		if seen[m.Name] {
			return m.Name, true
		}
		seen[m.Name] = true
		id = m.Causation
	}
	return "", false
}

// byTime lists every message of one correlation id in send order. The ids
// here are given out in send order, so sorting by id is sorting by time.
func byTime(all map[string]Msg, correlation string) []string {
	var out []string
	for id, m := range all {
		if m.Correlation == correlation {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func main() {
	fmt.Println("== Tab 1: three ids ==")
	root := Msg{"m-0", "POST /topup", "c-77", ""}
	paid := child(root, "m-1", "TopUpPaid")
	units := child(paid, "m-2", "UnitsGranted")
	ledger := child(paid, "m-3", "LedgerPosted")
	receipt := child(units, "m-4", "ReceiptSent")
	all := map[string]Msg{}
	for _, m := range []Msg{root, paid, units, ledger, receipt} {
		all[m.ID] = m
		fmt.Printf("%-13s id %s correlation %s caused by %q\n", m.Name, m.ID, m.Correlation, m.Causation)
	}
	fmt.Println("chain:", strings.Join(chain(all, "m-4"), " -> "))

	fmt.Println("== Tab 2: message id and duplicates ==")
	in := Inbox{}
	receipts := 0
	for _, id := range []string{"m-2", "m-2"} { // the broker delivers it twice
		if !in.Seen(id) {
			receipts++
		}
	}
	fmt.Printf("redelivered m-2: %d receipt\n", receipts)
	if !in.Seen("m-2b") { // a retry that made a fresh id
		receipts++
	}
	fmt.Printf("retry with new id m-2b: %d receipts\n", receipts)
	in2 := Inbox{}
	key, sent := "T-1043.UnitsGranted", 0
	for _, id := range []string{key, key} { // the id comes from the event
		if !in2.Seen(id) {
			sent++
		}
	}
	fmt.Printf("id from the event %s, retried: %d receipt\n", key, sent)

	fmt.Println("== Tab 3: causation tree ==")
	again := child(ledger, "m-5", "TopUpPaid") // the wallet service reacts to LedgerPosted
	all[again.ID] = again
	fmt.Println("flat list for c-77:", byTime(all, "c-77"))
	fmt.Println("chain of m-5:", strings.Join(chain(all, "m-5"), " -> "))
	if name, bad := hasCycle(all, "m-5"); bad {
		fmt.Printf("cycle: %s appears twice on one causation chain\n", name)
	}
}
