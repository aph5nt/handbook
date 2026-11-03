// One shop, two workloads. A disk that reads a fixed number of values a
// second is shared by everything that runs on one database. The program
// prints how long each piece of work takes and how long checkouts wait.
package main

import "fmt"

const (
	rows            = 1000000 // orders, standing for millions
	cols            = 50      // columns per order row
	valuesPerSecond = 1000000 // what the disk reads in a second
)

// DB is one database with one disk. busyUntil is when the disk is free.
type DB struct {
	name      string
	columnar  bool
	busyUntil float64
	segments  int
}

// run books the disk for a number of values, after whatever is queued.
func (d *DB) run(at float64, values int) (wait, took float64) {
	start := max(at, d.busyUntil)
	took = float64(values) / valuesPerSecond
	d.busyUntil = start + took
	return start - at, took
}

// checkout reads one customer by key and writes one order.
func (d *DB) checkout(at float64) (wait float64, values int) {
	values = 2 // row store: one piece read, one piece written
	if d.columnar {
		values = 2 * cols // one place per column file, for the read and the write
		d.segments++      // a one-row insert makes its own segment
	}
	wait, _ = d.run(at, values)
	return wait, values
}

// report is revenue per region per month: every row, two columns.
func (d *DB) report(at float64) (values int, took float64) {
	values = rows * cols // row store: whole rows come off disk
	if d.columnar {
		values = rows * 2 // column store: the region and amount files only
	}
	_, took = d.run(at, values)
	return values, took
}

func twoWorkloads() {
	shop := &DB{name: "checkout database"}
	_, v := shop.checkout(0)
	fmt.Printf("OLTP checkout: %d values, by key\n", v)
	dw := &DB{name: "warehouse", columnar: true}
	v, took := dw.report(0)
	fmt.Printf("OLAP report on the warehouse: %s values, %.0f s\n", comma(v), took)
}

func mixed() {
	shop := &DB{name: "checkout database"}
	v, took := shop.report(0) // the year report lands on the checkout database
	fmt.Printf("report on the checkout database: %s values, %.0f s\n", comma(v), took)
	for n := 1; n <= 4; n++ {
		wait, _ := shop.checkout(float64(n)) // a customer pays at second n
		fmt.Printf("checkout %d at %d s waits %.0f s\n", n, n, wait)
	}

	dw := &DB{name: "warehouse", columnar: true}
	for n := 0; n < 16; n++ {
		_, v = dw.checkout(0) // 16 orders, one row each
	}
	fmt.Printf("checkout on the warehouse: %d values per order, %d tiny segments\n", v, dw.segments)

	const ordersPerSecond = 1000
	replica := &DB{name: "read replica"}
	_, scan := replica.report(0) // the report moves to the replica
	behind := int(scan) * ordersPerSecond
	fmt.Printf("replica: replay waits for a %.0f s scan, %s orders behind\n", scan, comma(behind))
}

func twoSystems() {
	shop := &DB{name: "checkout database"}
	dw := &DB{name: "warehouse", columnar: true}
	_, took := dw.report(0) // the report runs on the copy
	wait, _ := shop.checkout(1)
	fmt.Printf("two systems: checkout waits %.0f s, report takes %.0f s on the warehouse\n", wait, took)
}

// comma writes 50000000 as 50,000,000.
func comma(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func main() {
	twoWorkloads()
	mixed()
	twoSystems()
}
