// Replays the CQRS and event sourcing lesson. The chain is the event log.
// The indexer folds it into warehouse tables (swaps by pool and day). The
// graph sync projects the warehouse transfers into a graph of links by
// address. Then a projection bug, a hand patch, and the rebuild.
package main

import (
	"fmt"
	"sort"
)

// Event is one chain event. Nothing is ever edited.
type Event struct {
	block    int
	kind     string // "transfer" or "swap"
	from, to string
	pool     string
	amount   int
	viaSwap  bool // a transfer made inside a swap transaction
}

var addrs = []string{"A", "B", "C", "D", "E", "F", "G", "H"}

// chain makes the same events every run: one swap and one transfer per block.
func chain(height int) []Event {
	var ev []Event
	for b := 1; b <= height; b++ {
		pool := "Q"
		if b%2 == 0 {
			pool = "P"
		}
		ev = append(ev, Event{block: b, kind: "swap", pool: pool, amount: b%5 + 1})
		from, to := addrs[b%8], addrs[(b%8+1+(b/8)%2)%8] // money moves one or two steps on
		ev = append(ev, Event{block: b, kind: "transfer", from: from, to: to, amount: b%9 + 1, viaSwap: b%3 == 0})
	}
	return ev
}

func day(block int) string {
	if block <= 60 {
		return "2026-10-05"
	}
	return "2026-10-06"
}

// Warehouse is read model 1: rows shaped for "swaps of a pool on a day".
type Warehouse struct {
	swaps      map[string][]Event // key pool|day
	transfers  []Event
	checkpoint int
}

// fold is the indexer: state is a fold over the events.
func (w *Warehouse) fold(ev []Event, to int) {
	for _, e := range ev {
		if e.block <= w.checkpoint || e.block > to {
			continue
		}
		if e.kind == "swap" {
			k := e.pool + "|" + day(e.block) // one key per pool and day
			w.swaps[k] = append(w.swaps[k], e)
		} else {
			w.transfers = append(w.transfers, e)
		}
	}
	w.checkpoint = to // folded up to here
}

// Graph is read model 2: links by address, shaped for "where did it go".
type Graph struct {
	links    map[string]map[string]int // from -> to -> amount
	position int
	bug      bool // the projection rule counts swap transfers twice
}

// project is the graph sync: warehouse transfers after its position.
func (g *Graph) project(w *Warehouse, to int) {
	for _, t := range w.transfers {
		if t.block <= g.position || t.block > to {
			continue
		}
		g.links[t.from][t.to] += t.amount
		if g.bug && t.viaSwap {
			g.links[t.from][t.to] += t.amount // the bug: counted twice
		}
	}
	g.position = to
}

func (g *Graph) sent(a string) int {
	n := 0
	for _, v := range g.links[a] {
		n += v
	}
	return n
}

// truthSent is the warehouse answer, used to check the graph.
func (w *Warehouse) truthSent(a string, upTo int) int {
	n := 0
	for _, t := range w.transfers {
		if t.from == a && t.block <= upTo {
			n += t.amount
		}
	}
	return n
}

// newGraph is an empty graph with a link map for every address.
func newGraph(bug bool) *Graph {
	g := &Graph{links: map[string]map[string]int{}, bug: bug}
	for _, a := range addrs {
		g.links[a] = map[string]int{}
	}
	return g
}

// rebuild drops the graph and projects again from the warehouse tables.
func rebuild(w *Warehouse) *Graph {
	g := newGraph(false)       // the old graph is dropped, the rule is fixed
	g.project(w, w.checkpoint) // from the first warehouse row
	return g
}

// walk follows links hop by hop: one map lookup per hop, no join.
func (g *Graph) walk(a string, hops int) (out [][]string) {
	seen, frontier := map[string]bool{a: true}, []string{a}
	for h := 0; h < hops; h++ {
		var next []string
		for _, f := range frontier {
			for to := range g.links[f] { // one lookup per hop, no join
				if !seen[to] {
					seen[to], next = true, append(next, to)
				}
			}
		}
		out, frontier = append(out, next), next
	}
	return
}

func receivers(g *Graph, a string) []string {
	var out []string
	for to, v := range g.links[a] {
		out = append(out, fmt.Sprintf("%s:%d", to, v))
	}
	sort.Strings(out)
	return out
}

func main() {
	ev := chain(300)
	fmt.Println("tab 1: two read models")
	w := &Warehouse{swaps: map[string][]Event{}}
	w.fold(ev, 120)
	all := 0
	for _, rows := range w.swaps {
		all += len(rows)
	}
	fmt.Printf("  indexer folded blocks 1 to %d: %d swap rows, %d transfer rows\n", w.checkpoint, all, len(w.transfers))
	fmt.Printf("  swaps of pool P on 2026-10-06: read %d rows of %d\n", len(w.swaps["P|2026-10-06"]), all)
	g := newGraph(false)
	g.project(w, 112)
	fmt.Printf("  money from A, links: %v\n", receivers(g, "A"))
	for h, addrs := range g.walk("A", 2) {
		sort.Strings(addrs)
		fmt.Printf("  where did it go, hop %d: %v\n", h+1, addrs)
	}
	fmt.Printf("  graph sync position %d, indexer %d: graph is %d blocks behind\n", g.position, w.checkpoint, w.checkpoint-g.position)

	fmt.Println("tab 2: a projection bug")
	g = newGraph(true)
	g.project(w, 120)
	fmt.Printf("  A sent: warehouse %d, graph %d\n", w.truthSent("A", 120), g.sent("A"))
	// hand patch: a one-off script takes the extra off the links that exist now
	for _, t := range w.transfers {
		if t.viaSwap && t.block <= g.position {
			g.links[t.from][t.to] -= t.amount
		}
	}
	fmt.Printf("  after the hand patch: graph %d\n", g.sent("A"))
	w.fold(ev, 150)
	g.project(w, 150) // same wrong rule over new blocks
	fmt.Printf("  next sync to 150: warehouse %d, graph %d\n", w.truthSent("A", 150), g.sent("A"))
	// fix the rule, drop the graph, rebuild from the tables
	g = rebuild(w)
	fmt.Printf("  rule fixed, graph rebuilt from %d transfer rows: warehouse %d, graph %d\n", len(w.transfers), w.truthSent("A", 150), g.sent("A"))
	for _, h := range []int{150, 300} {
		fmt.Printf("  replay from block 1 at height %d: %d events to decode\n", h, len(chain(h)))
	}
}
