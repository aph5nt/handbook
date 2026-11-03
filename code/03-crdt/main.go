// CRDT: a G-Counter of likes in two regions that merges with no leader, and
// the same counter used for stock, which converges to -1. It replays the
// lesson, then shows the fix: one leader decides stock.
package main

import "fmt"

// GCounter has one slot per replica. A replica only adds to its own slot.
type GCounter map[string]int

// Inc adds one to this replica's own slot.
func (c GCounter) Inc(me string) {
	c[me]++
}

// Value is the sum of all slots.
func (c GCounter) Value() int {
	total := 0
	for _, v := range c {
		total += v
	}
	return total
}

// Merge takes the larger value of each slot. It is commutative,
// associative and idempotent, so any order and any repeat agree.
func (c GCounter) Merge(other GCounter) {
	for r, v := range other {
		if v > c[r] {
			c[r] = v
		}
	}
}

// copyOf is the state a replica sends to its peer.
func copyOf(c GCounter) GCounter {
	out := GCounter{}
	for r, v := range c {
		out[r] = v
	}
	return out
}

// SellLocal sells if this replica's own view of stock is above zero.
// The check only sees what this replica knows.
func SellLocal(sold GCounter, me string, stock int) bool {
	if stock-sold.Value() <= 0 {
		return false
	}
	sold.Inc(me)
	return true
}

// StockLeader is the fix: one place holds stock and refuses a second sale.
type StockLeader struct{ stock int }

// Sell decrements stock only if there is some left.
func (l *StockLeader) Sell() bool {
	if l.stock <= 0 {
		return false
	}
	l.stock--
	return true
}

func main() {
	fmt.Println("== Likes: two replicas, link down")
	eu, us := GCounter{"EU": 0, "US": 0}, GCounter{"EU": 0, "US": 0}
	for i := 0; i < 3; i++ {
		eu.Inc("EU")
	}
	for i := 0; i < 2; i++ {
		us.Inc("US")
	}
	fmt.Printf("while down: Europe total %d, US total %d\n", eu.Value(), us.Value())
	euState, usState := copyOf(eu), copyOf(us)
	eu.Merge(usState)
	us.Merge(euState)
	fmt.Printf("after merge: Europe EU %d US %d total %d, US EU %d US %d total %d\n", eu["EU"], eu["US"], eu.Value(), us["EU"], us["US"], us.Value())
	us.Merge(euState) // a retry delivers the same state again
	fmt.Printf("merge again: total still %d\n", us.Value())

	fmt.Println("== Stock: 1 phone, sales counted with a CRDT")
	soldEU, soldUS := GCounter{}, GCounter{}
	fmt.Printf("Europe sells: %v, US sells: %v\n", SellLocal(soldEU, "EU", 1), SellLocal(soldUS, "US", 1))
	soldEU.Merge(copyOf(soldUS))
	fmt.Printf("after merge: sold %d, stock %d\n", soldEU.Value(), 1-soldEU.Value())

	fmt.Println("== Fix: one leader for stock")
	l := &StockLeader{stock: 1}
	fmt.Printf("Europe sells: %v, US sells: %v, stock %d\n", l.Sell(), l.Sell(), l.stock)
}
