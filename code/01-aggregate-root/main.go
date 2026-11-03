// Aggregate root: one door. The Order checks its rule before anything changes,
// saves as one unit, then publishes an event. Others update a moment later.
package main

import (
	"errors"
	"fmt"
)

// line is private: nobody outside the aggregate can edit it.
type line struct {
	sku, qty, price int
}

func (o *Order) total() int {
	t := 0
	for _, l := range o.lines {
		t += l.qty * l.price
	}
	return t
}

// Order is the aggregate root.
type Order struct {
	ID     int
	limit  int
	lines  []line
	events []string
}

// AddLine is the only door. It checks the invariant before any change.
func (o *Order) AddLine(sku, qty, price int) error {
	next := o.total() + qty*price
	if next > o.limit {
		return errors.New("rejected: over limit") // nothing happened
	}
	o.lines = append(o.lines, line{sku, qty, price})
	o.events = append(o.events, fmt.Sprintf("OrderLineAdded order %d plus %d", o.ID, qty*price))
	return nil
}

// Store saves one whole aggregate in one transaction.
type Store struct {
	Saved     map[int]Order
	Published []string
}

func (s *Store) Save(o *Order) {
	s.Saved[o.ID] = *o                             // the whole Order, or the old one stays
	s.Published = append(s.Published, o.events...) // after the commit
	o.events = nil
}

// Customer is another aggregate. A handler updates it in its own transaction.
type Customer struct{ Used int }

func (c *Customer) OnLineAdded(amount int) { c.Used += amount }

// RowLock is the failure: Order and Customer saved in one transaction,
// so every order of this customer needs this one row.
type RowLock struct{ Holder int }

func (l *RowLock) SaveBoth(orderID int) bool {
	if l.Holder != 0 {
		return false // waits for the holder to commit
	}
	l.Holder = orderID
	return true
}

func main() {
	o := &Order{ID: 1001, limit: 400, lines: []line{{7, 1, 100}, {9, 1, 80}}}
	c := &Customer{Used: 180}
	s := &Store{Saved: map[int]Order{}}
	fmt.Println("total", o.total(), "limit", o.limit)
	fmt.Println("AddLine qty 3:", o.AddLine(42, 3, 90), "- total", o.total())
	fmt.Println("AddLine qty 2:", o.AddLine(42, 2, 90), "- total", o.total())
	s.Save(o)
	fmt.Println("saved lines:", len(s.Saved[1001].lines), "event:", s.Published[0])
	fmt.Println("customer used, before the handler:", c.Used)
	c.OnLineAdded(180)
	fmt.Println("customer used, a moment later:", c.Used)
	// the failure: one transaction over Order and Customer
	row := &RowLock{}
	for _, id := range []int{1001, 1002, 1003} {
		fmt.Println("order", id, "got the Customer row:", row.SaveBoth(id), "- holder", row.Holder)
	}
}
