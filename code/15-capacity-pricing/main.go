// The bill mix: reserved for the floor, on-demand for the bump, spot for the
// batch. Prices are made-up examples to show the shape, not a quote.
package main

import "fmt"

const (
	onDemand = 1.00 // price per core hour
	reserved = 0.60 // paid every hour, busy or idle
	spot     = 0.30
)

type Shape struct {
	FloorCores, BumpCores, BumpHours, BatchCores, BatchHours float64
}

// allOnDemand is the bill with no choices made, per day.
func allOnDemand(s Shape) float64 {
	return (s.FloorCores*24 + s.BumpCores*s.BumpHours + s.BatchCores*s.BatchHours) * onDemand
}

// mix fits a price to each part, per day. A reservation above the
// floor is paid for and sits idle.
func mix(s Shape, reservedCores float64) float64 {
	return reservedCores*24*reserved + s.BumpCores*s.BumpHours*onDemand + s.BatchCores*s.BatchHours*spot
}

// Task is one batch task, ten chunks long.
type Task struct{ Chunk, Saved int }

// notice is the cloud's warning that it takes the spot node back.
var notice = make(chan bool, 1)

// doChunk does one chunk. Here the node is taken back after chunk 3.
func doChunk(n int) {
	if n == 3 {
		notice <- true
	}
}

// run works chunk by chunk and saves a checkpoint after each one.
func (t *Task) run() {
	for t.Chunk = t.Saved + 1; t.Chunk <= 10; t.Chunk++ {
		select {
		case <-notice: // the warning: stop cleanly, the checkpoint is saved
			return
		default:
		}
		doChunk(t.Chunk)
		t.Saved = t.Chunk // the checkpoint
	}
}

// resume restarts the task on another node.
func resume(t *Task, spotFree, checkpoint bool) {
	pool := "spot"
	if !spotFree {
		pool = "the on-demand fallback pool" // no spot capacity left
	}
	if !checkpoint {
		t.Saved = 0 // no checkpoint: chunk 1 again
	}
	fmt.Printf("resume on %s at chunk %d of 10\n", pool, t.Saved+1)
}

func main() {
	s := Shape{FloorCores: 10, BumpCores: 6, BumpHours: 10, BatchCores: 20, BatchHours: 6}
	fmt.Printf("all on-demand: %.0f a day\n", allOnDemand(s))
	fmt.Printf("fitted mix, 10 cores reserved: %.0f a day\n", mix(s, 10))
	over := mix(s, 16)
	fmt.Printf("16 cores reserved for a peak that never came: %.0f a day, %.0f of it for idle cores\n", over, 6*24*reserved)

	t := &Task{}
	t.run()
	fmt.Print("reclaimed after chunk 3, with a checkpoint: ")
	resume(t, false, true)
	t = &Task{}
	t.run()
	fmt.Print("reclaimed after chunk 3, no checkpoint: ")
	resume(t, false, false)
}
