// Two systems with a nightly copy, then one HTAP system that keeps a row
// copy and a column copy fed from one log. The program prints how fresh
// each report is and how long an order write waits.
package main

import (
	"fmt"
	"time"
)

// at is a time on 1 October 2026; day -1 is 30 September.
func at(day, h, m, s int) time.Time { return time.Date(2026, 10, 1+day, h, m, s, 0, time.UTC) }

type Order struct {
	id int
	at time.Time
}

// Store is one copy of the orders table.
type Store struct {
	name   string
	orders []Order
}

func (s *Store) Write(o Order) { s.orders = append(s.orders, o) }

func (s *Store) Newest() Order { return s.orders[len(s.orders)-1] }

func (s *Store) Has(id int) bool {
	for _, o := range s.orders {
		if o.id == id {
			return true
		}
	}
	return false
}

// nightlyCopy is the pipeline: it copies every order written before it runs.
func nightlyCopy(from, to *Store, now time.Time) {
	for _, o := range from.orders {
		if o.at.Before(now) && !to.Has(o.id) {
			to.Write(o)
		}
	}
}

func classic() {
	pg, dw := &Store{name: "PostgreSQL"}, &Store{name: "warehouse"}
	pg.Write(Order{500, at(-1, 18, 0, 0)}) // yesterday's last order
	nightlyCopy(pg, dw, at(0, 2, 0, 0))    // the 02:00 copy job
	pg.Write(Order{501, at(0, 9, 0, 0)})   // order 501 at 09:00
	n := dw.Newest()                       // the 10:00 report reads the warehouse
	fmt.Printf("classic, 10:00 report: newest order %d from %s, has 501: %v\n", n.id, n.at.Format("2 Jan 15:04"), dw.Has(501))
	fmt.Printf("classic, report on PostgreSQL instead: has 501: %v, but the scan shares the app's disk\n", pg.Has(501))
}

// Node is one machine. busyUntil is when its CPU and disk are free.
type Node struct{ busyUntil time.Time }

func (n *Node) run(now time.Time, d time.Duration) (wait time.Duration) {
	start := now
	if n.busyUntil.After(start) {
		start = n.busyUntil
	}
	n.busyUntil = start.Add(d)
	return start.Sub(now)
}

// HTAP keeps a row copy and a column copy. The log feeds the column copy.
type HTAP struct {
	row, col         Store
	log              []Order
	lag              time.Duration // how far the column copy runs behind
	rowNode, colNode *Node
}

// Write goes to the row copy and the log, like any OLTP database.
func (h *HTAP) Write(o Order) time.Duration {
	h.row.Write(o)
	h.log = append(h.log, o)
	return h.rowNode.run(o.at, time.Millisecond)
}

// apply replays the log into the column copy, up to now minus the lag.
func (h *HTAP) apply(now time.Time) {
	for len(h.log) > 0 && !h.log[0].at.Add(h.lag).After(now) {
		h.col.Write(h.log[0])
		h.log = h.log[1:]
	}
}

// route is the planner: a lookup by key goes to the row copy, a scan to the column copy.
func route(query string) string {
	if query == "lookup" {
		return "row copy"
	}
	return "column copy"
}

func htap() {
	h := &HTAP{lag: 2 * time.Second, rowNode: &Node{}}
	h.colNode = h.rowNode // first: both copies on the same nodes
	h.Write(Order{500, at(-1, 18, 0, 0)})
	h.Write(Order{501, at(0, 9, 0, 0)})
	h.apply(at(0, 9, 0, 1))
	fmt.Printf("htap, 09:00:01: column copy has 501: %v\n", h.col.Has(501))
	h.apply(at(0, 9, 0, 2))
	fmt.Printf("htap, 09:00:02: column copy has 501: %v (lag %v)\n", h.col.Has(501), h.lag)
	fmt.Printf("htap planner: lookup of order 501 -> %s, revenue per day -> %s\n", route("lookup"), route("scan"))
	fmt.Printf("htap, 10:00 report on the column copy: has 501: %v\n", h.col.Has(501))

	h.colNode.run(at(0, 10, 0, 0), 50*time.Second) // a heavy year-end scan
	wait := h.Write(Order{502, at(0, 10, 0, 1)})
	fmt.Printf("shared nodes: order 502 at 10:00:01 waits %v behind the scan\n", wait)

	h.colNode = &Node{} // fix: column copies on their own nodes
	h.colNode.run(at(0, 10, 5, 0), 50*time.Second)
	wait = h.Write(Order{503, at(0, 10, 5, 1)})
	fmt.Printf("separate nodes: order 503 at 10:05:01 waits %v\n", wait)
}

func main() {
	classic()
	htap()
}
