// Write-behind: the cache says "saved" at once and a flusher writes later.
// Replays the page-view counter: 3,000 views flushed, 2,500 lost in a crash,
// then the same crash with a durable log in front.
package main

import "fmt"

// Counter is Redis holding the unflushed count.
type Counter struct {
	unflushed int
	log       []int // durable append-only log, nil when switched off
	useLog    bool
}

// Incr acknowledges at once. The warehouse has not seen the view.
func (c *Counter) Incr() {
	if c.useLog {
		c.log = append(c.log, 1) // the fix: write the view to disk first
	}
	c.unflushed++
}

// Warehouse is StarRocks. Batches carry an id, so a retry is written once.
type Warehouse struct {
	rows map[int]int // batch id -> views
}

func (w *Warehouse) Insert(batchID, views int) {
	if _, seen := w.rows[batchID]; seen {
		return // the same batch again: ignore it
	}
	w.rows[batchID] = views
}

func (w *Warehouse) Total() (t int) {
	for _, v := range w.rows {
		t += v
	}
	return t
}

// flush runs once a minute: read the count, reset it, write one row.
func flush(c *Counter, w *Warehouse, batchID int) {
	n := c.unflushed
	c.unflushed = 0
	w.Insert(batchID, n)
	c.log = c.log[:0] // flushed views no longer need replaying
}

// crash loses everything that lives only in memory.
func crash(c *Counter) {
	c.unflushed = 0
	if c.useLog {
		c.unflushed = len(c.log) // replay the log after the restart
	}
}

func run(useLog bool) {
	c := &Counter{useLog: useLog}
	w := &Warehouse{rows: map[int]int{}}
	for i := 0; i < 3000; i++ {
		c.Incr()
	}
	fmt.Printf("  minute 1: unflushed in Redis = %d, warehouse = %d\n", c.unflushed, w.Total())
	flush(c, w, 1)
	fmt.Printf("  flush batch 1: unflushed = %d, warehouse = %d (one insert)\n", c.unflushed, w.Total())
	for i := 0; i < 2500; i++ {
		c.Incr()
	}
	fmt.Printf("  40 s later: unflushed = %d, all acknowledged to the website\n", c.unflushed)
	crash(c)
	fmt.Printf("  Redis dies at second 50, restarts: unflushed = %d\n", c.unflushed)
	flush(c, w, 2)
	flush(c, w, 2) // a retried batch: written once
	fmt.Printf("  flush batch 2 (sent twice): warehouse = %d of 5,500 views\n", w.Total())
}

func main() {
	fmt.Println("== write-behind, no log ==")
	run(false)
	fmt.Println("== write-behind with a durable log in front ==")
	run(true)
}
