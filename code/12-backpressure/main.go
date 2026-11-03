// Command backpressure replays the lesson: a reader fetches blocks at 10 a second,
// a writer stores 6 a second, and a queue sits between them. With a bounded queue
// (a Go buffered channel of 4) the full queue makes the reader wait. With no bound
// the queue grows until the process runs out of memory.
//
// Time moves in ticks of 1/60 s, in one goroutine, so every run prints the same numbers.
package main

import "fmt"

const ticksPerSec = 60

// Bounded runs the pipeline for secs seconds on a channel of capacity 4.
// readEvery and writeEvery are the ticks between one block and the next.
func Bounded(q chan int, secs, readEvery, writeEvery int, next *int) (read, written, waits int) {
	pending := false // the reader holds a block it could not put in the queue yet
	for tick := 1; tick <= secs*ticksPerSec; tick++ {
		if tick%writeEvery == 0 {
			select {
			case <-q: // the writer takes one block out and stores it
				written++
			default: // queue empty: the writer idles
			}
		}
		if pending || tick%readEvery == 0 {
			select {
			case q <- *next: // room in the queue: the send succeeds
				*next++
				read++
				pending = false
			default: // queue full: the send would block, so the reader waits
				pending = true
				waits++
			}
		}
	}
	return
}

// Unbounded has no limit: the reader appends and never waits.
// It returns the queue length after secs seconds, or the second it hit the memory limit.
func Unbounded(secs int, readPerSec, writePerSec, limitBlocks int) (queued, killedAt int) {
	for s := 1; s <= secs; s++ {
		queued += readPerSec - writePerSec // 4 more arrive than leave, every second
		if queued >= limitBlocks {
			return queued, s // out of memory: the process is killed, the queue is lost
		}
	}
	return queued, 0
}

func main() {
	q := make(chan int, 4) // the bound: at most 4 blocks wait
	next := 9506           // the next block number to read

	r, w, waits := Bounded(q, 1, 6, 10, &next)
	fmt.Printf("bounded, second 1: read %d, written %d, queue %d of %d\n", r, w, len(q), cap(q))

	r, w, waits = Bounded(q, 10, 6, 10, &next)
	fmt.Printf("bounded, seconds 2-11: read %d (%.0f a second), written %d, queue %d, reader waited %.1f s of 10\n",
		r, float64(r)/10, w, len(q), float64(waits)/ticksPerSec)

	r, w, waits = Bounded(q, 10, 6, 5, &next) // compaction done: the writer does 12 a second
	fmt.Printf("writer at 12 a second, 10 s: read %d (%.0f a second), written %d, queue %d, reader waited %.1f s\n",
		r, float64(r)/10, w, len(q), float64(waits)/ticksPerSec)

	const blockMiB = 0.5
	limit := 14400 // blocks: 14,400 x 0.5 MiB = 7,200 MiB, the pod's memory limit
	n, _ := Unbounded(60, 10, 6, limit)
	fmt.Printf("\nunbounded after 1 minute: %d blocks queued, the newest waits %d s\n", n, n/6)
	n, _ = Unbounded(600, 10, 6, limit)
	fmt.Printf("unbounded after 10 minutes: %d blocks queued, the newest waits %d s\n", n, n/6)
	n, killed := Unbounded(7200, 10, 6, limit)
	fmt.Printf("unbounded: %d blocks = %.0f MiB at second %d: out of memory, all %d lost\n",
		n, float64(n)*blockMiB, killed, n)
}
