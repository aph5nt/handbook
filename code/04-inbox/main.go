// The inbox: effect and event id in one transaction.
//
// Maps stand in for the shipment and inbox tables. tx() applies both writes
// together, the way one database transaction does. A crash is modelled by
// stopping before the next line runs. Deterministic.
package main

import "fmt"

type DB struct {
	inbox     map[string]int // event id -> day it was stored
	shipments map[string]int // event id -> shipments made
}

func newDB() *DB { return &DB{map[string]int{}, map[string]int{}} }

// handle is the correct consumer: inbox row and effect in one transaction.
func (d *DB) handle(id string, day int) string {
	if _, seen := d.inbox[id]; seen {
		return "duplicate: rolled back, skipped" // unique key on the event id
	}
	d.inbox[id] = day
	d.shipments[id]++
	return "committed"
}

// cleanup deletes inbox rows older than the given number of days.
func (d *DB) cleanup(today, keepDays int) {
	for id, day := range d.inbox {
		if today-day > keepDays {
			delete(d.inbox, id)
		}
	}
}

func main() {
	fmt.Println("== Tab 1: same transaction ==")
	d := newDB()
	fmt.Println("evt-501:", d.handle("evt-501", 1))
	fmt.Println("crash before the offset commit, redelivery of evt-501:", d.handle("evt-501", 1))
	fmt.Printf("evt-501 shipped %d time\n", d.shipments["evt-501"])

	fmt.Println("== Tab 2: split into two writes ==")
	d = newDB()
	d.shipments["evt-601"]++ // effect first, committed
	// crash here: the inbox row was never written
	if _, seen := d.inbox["evt-601"]; !seen {
		d.shipments["evt-601"]++ // redelivered, no row: shipped again
	}
	fmt.Printf("effect first: evt-601 shipped %d times\n", d.shipments["evt-601"])
	d = newDB()
	d.inbox["evt-602"] = 1 // inbox first, committed
	// crash here: the shipment was never written
	_, seen := d.inbox["evt-602"]
	fmt.Printf("inbox first: evt-602 seen=%v on redelivery, skipped, shipped %d times\n", seen, d.shipments["evt-602"])
	d = newDB()
	fmt.Println("the fix, one transaction:", d.handle("evt-603", 30))

	fmt.Println("== Tab 3: keeping it clean ==")
	d = newDB()
	for day := 1; day <= 30; day++ {
		d.handle(fmt.Sprintf("evt-day-%d", day), day)
	}
	d.cleanup(30, 8) // retention 7 days + 1 day margin
	fmt.Printf("cleanup at 8 days: %d of 30 rows kept\n", len(d.inbox))
	fmt.Println("replay from day 25 with 8 days kept:", d.handle("evt-day-25", 30))
	d.cleanup(30, 1) // the trap
	fmt.Println("replay from day 25 after cleanup at 1 day:", d.handle("evt-day-25", 30))
	fmt.Printf("evt-day-25 shipped %d times\n", d.shipments["evt-day-25"])

	// An inbox of one row: strict order per partition.
	partition := 3
	last := map[int]int{partition: 90}
	off := 90
	duplicate := off <= last[partition]
	fmt.Printf("last offset per partition: offset %d <= last %d: duplicate=%v\n", off, last[partition], duplicate)
}
