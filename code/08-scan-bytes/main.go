// An in-memory model of a warehouse bill. The compressed size of each column
// file per day is an assumed example, not a measurement. The sum is the point:
// scan bytes = compressed size of the named columns, over the partitions read.
package main

import "fmt"

var sizeMB = map[string]int{
	"day": 1, "pool": 8, "amount": 20, "sender": 60, "to": 60,
	"tx_hash": 120, "fee": 10, "input_data": 900,
}

var all = []string{"day", "pool", "amount", "sender", "to", "tx_hash", "fee", "input_data"}

const year = 365

func scanMB(cols []string, partitions int) int {
	total := 0
	for _, c := range cols {
		total += sizeMB[c] * partitions
	}
	return total
}

func show(label string, mb int) {
	if mb >= 1000 {
		fmt.Printf("%-48s %8d MB (%.1f GB)\n", label, mb, float64(mb)/1000)
		return
	}
	fmt.Printf("%-48s %8d MB\n", label, mb)
}

func main() {
	three := []string{"day", "pool", "amount"}
	show("SELECT * over a year", scanMB(all, year))
	show("day, pool, amount over a year", scanMB(three, year))
	show("day, pool, amount, one day", scanMB(three, 1))
	show("the same plus input_data, one day", scanMB(append(three[:3:3], "input_data"), 1))

	// LIMIT cuts the rows returned. The files are opened first, so it cuts no bytes here.
	limitRows := 100
	fmt.Printf("SELECT * LIMIT %d: %d rows returned, %d MB read\n", limitRows, limitRows, scanMB(all, year))

	// A guard rail refuses the query before any byte is read.
	hasPartitionFilter := false
	if !hasPartitionFilter {
		fmt.Println("require_partition_filter = true: refused, 0 MB read")
	}
}
