// Projection: the graph sync folds transfer rows into money links.
// It replays the lesson: two batches, a crash between the two writes,
// the doubled totals, and the idempotent fix.
package main

import "fmt"

type Transfer struct {
	ID       string
	Block    int
	From, To string
	Amount   int // whole ETH, to keep the story readable
}

// Graph is the read model: money links plus the ids already folded.
type Graph struct {
	Links map[string]int
	Seen  map[string]bool
}

// Warehouse holds the source rows and the checkpoint row graph_sync.
type Warehouse struct {
	Rows       []Transfer
	Checkpoint int
}

func (w *Warehouse) After(block, limit int) []Transfer {
	var out []Transfer
	for _, r := range w.Rows {
		if r.Block > block && len(out) < limit {
			out = append(out, r)
		}
	}
	return out
}

// Step runs one batch. crash stops it between the two writes.
// idempotent switches the seen check on or off.
func Step(w *Warehouse, g *Graph, idempotent, crash bool) bool {
	cp := w.Checkpoint     // load the bookmark
	rows := w.After(cp, 3) // the next batch, in block order
	if len(rows) == 0 {
		return false
	}
	for _, r := range rows { // write 1: the graph
		if idempotent && g.Seen[r.ID] {
			continue // folded before: skip
		}
		g.Seen[r.ID] = true
		g.Links[r.From+" -> "+r.To] += r.Amount
	}
	if crash {
		return false // the process dies before write 2
	}
	w.Checkpoint = rows[len(rows)-1].Block // write 2: another database
	return true
}

// Rebuild starts the projection again from block 1 under a new rule.
func Rebuild(w *Warehouse, g *Graph) {
	g.Links, g.Seen = map[string]int{}, map[string]bool{}
	w.Checkpoint = 0 // replay from the first row
}

func rows() []Transfer {
	return []Transfer{
		{"t100", 100, "A", "B", 1}, {"t101", 101, "A", "B", 2}, {"t102", 102, "A", "C", 1},
		{"t103", 103, "A", "B", 3}, {"t104", 104, "B", "C", 1}, {"t105", 105, "A", "B", 1},
	}
}

func show(label string, w *Warehouse, g *Graph) {
	fmt.Printf("%-34s A->B %2d  A->C %d  B->C %d  checkpoint %d\n",
		label, g.Links["A -> B"], g.Links["A -> C"], g.Links["B -> C"], w.Checkpoint)
}

func run(idempotent bool) {
	w := &Warehouse{Rows: rows(), Checkpoint: 99}
	g := &Graph{Links: map[string]int{}, Seen: map[string]bool{}}
	show("start", w, g)
	Step(w, g, idempotent, false)
	show("batch 100-102", w, g)
	Step(w, g, idempotent, true)
	show("batch 103-105, crash before cp", w, g)
	Step(w, g, idempotent, false)
	show("restart, replay 103-105", w, g)
}

func main() {
	fmt.Println("seen check OFF (adding projector):")
	run(false)
	fmt.Println("seen check ON (idempotent projector):")
	run(true)
	// rebuild: clear the read model, checkpoint 0, replay everything
	w := &Warehouse{Rows: rows(), Checkpoint: 105}
	g := &Graph{Links: map[string]int{"A -> B": 99}, Seen: map[string]bool{}}
	Rebuild(w, g)
	for Step(w, g, true, false) {
	}
	show("rebuild from block 0", w, g)
}
