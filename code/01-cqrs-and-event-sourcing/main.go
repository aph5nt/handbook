// CQRS: commands change the write model, queries read a read model that a
// projector keeps up to date. Event sourcing: the state is a fold of events.
package main

import "fmt"

// ---- CQRS ----

// Write is the write model: the warehouse tables. Read is the graph.
type Write struct{ Block int }
type Read struct{ Block int }

// Command: IndexBlock. Only the write model changes, at once.
func IndexBlock(w *Write, b int) { w.Block = b }

// Project is the graph sync: read what is new, update the graph, remember how far.
func Project(w *Write, r *Read, max int) {
	for r.Block < w.Block && max > 0 {
		r.Block++ // fold the rows of block r.Block into links
		max--
	}
}

// Query reads the graph only, and says how fresh its answer is.
func Query(w *Write, r *Read, block int, showLag bool) string {
	if block > r.Block {
		if showLag {
			return fmt.Sprintf("not in the graph yet: graph is at block %d, %d behind", r.Block, w.Block-r.Block)
		}
		return "no outgoing money" // stale, and believed
	}
	return "two hops, then into an exchange"
}

// ---- Event sourcing ----

type Event struct {
	Version int
	Kind    string // "Received" or "Sent"
	ETH     int
}

// Upcast turns an old v1 event into v2 when it is read. The store is never edited.
func Upcast(e Event) Event {
	if e.Version == 1 {
		e.Version = 2 // v1 had no token; it only ever meant ETH
	}
	return e
}

// Fold replays events from a starting state. bug4 is the projector bug.
func Fold(start int, events []Event, bug4 bool) int {
	bal := start
	for k, e := range events {
		e = Upcast(e)
		amt := e.ETH
		if bug4 && k == 3 {
			amt = 20 // read event 4 as 20 ETH sent
		}
		if e.Kind == "Sent" {
			amt = -amt
		}
		bal += amt
	}
	return bal
}

func main() {
	w, r := &Write{Block: 1004}, &Read{Block: 1004}
	IndexBlock(w, 1005)
	fmt.Println("after IndexBlock 1005: write", w.Block, "read", r.Block)
	Project(w, r, 10)
	fmt.Println("after projector: read", r.Block)
	for b := 1006; b <= 1009; b++ {
		IndexBlock(w, b)
	}
	fmt.Println("block 1008, no lag shown:", Query(w, r, 1008, false))
	fmt.Println("block 1008, lag shown:", Query(w, r, 1008, true))
	Project(w, r, 10)
	fmt.Println("caught up at", r.Block, ":", Query(w, r, 1008, true))

	ev := []Event{{1, "Received", 10}, {2, "Sent", 3}, {2, "Received", 5}, {2, "Sent", 2}, {2, "Received", 4}, {2, "Sent", 1}}
	fmt.Println("balance after 3 events:", Fold(0, ev[:3], false))
	fmt.Println("balance after 6 events:", Fold(0, ev, false))
	fmt.Println("projector bug:", Fold(0, ev, true))
	fmt.Println("rebuilt from event 1:", Fold(0, ev, false))
	snap := Fold(0, ev[:4], false)
	fmt.Println("snapshot after event 4:", snap)
	fmt.Println("from the snapshot, 2 events read:", Fold(snap, ev[4:], false))
	fmt.Println("event 1 upcast to version", Upcast(ev[0]).Version)
}
