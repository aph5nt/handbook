// Back-pressure: push with no limit, prefetch, and a pull loop.
//
// Plain numbers stand in for a RabbitMQ queue and its workers, and the pull
// tab uses a buffered channel of size 1 as the page in flight: the sync
// cannot read a page while the last one is not committed. Sizes, rates and
// limits are this example's. Deterministic.
package main

import "fmt"

const (
	jobMB    = 2    // memory one unacked job takes in a worker
	memoryMB = 8000 // a worker's limit
)

// push gives all it can to the first connected worker when prefetch is 0.
func push(jobs, prefetch int) (held int) {
	if prefetch == 0 || prefetch > jobs {
		return jobs
	}
	return prefetch
}

func main() {
	fmt.Println("== Tab 1: push with no limit ==")
	held := push(5000, 0)
	fmt.Printf("worker 1 holds %d unacked jobs = %d MB, limit %d MB\n", held, held*jobMB, memoryMB)
	if held*jobMB > memoryMB {
		fmt.Printf("out of memory: %d jobs go back to the queue\n", held)
		held = push(5000, 0)
		fmt.Printf("pushed to worker 2: holds %d\n", held)
	}

	fmt.Println("== Tab 2: prefetch 1 ==")
	queue := 5000
	for w := 1; w <= 3; w++ {
		queue -= push(queue, 1)
	}
	fmt.Printf("3 workers hold 1 each = %d MB each, queue %d\n", 1*jobMB, queue)
	unacked := 1
	unacked-- // worker 1 acks: the check passes again
	if unacked < 1 {
		queue -= push(queue, 1)
		fmt.Printf("worker 1 acks: unacked %d, next job sent, queue %d\n", unacked, queue)
	}
	queue = 4990
	const arrive = 5 // jobs per minute
	workers := 3
	minutes := 0
	for queue < 9000 {
		queue += arrive - workers // each worker finishes 1 a minute
		minutes++
	}
	fmt.Printf("arrivals %d a minute, 3 workers: queue reaches %d after %d minutes, workers healthy\n", arrive, queue, minutes)
	before := queue
	workers = 6
	queue += arrive - workers
	fmt.Printf("6 workers: queue %d to %d: shrinking\n", before, queue)

	fmt.Println("== Tab 3: pull ==")
	inFlight := make(chan int, 1) // room for one page: that is the back-pressure
	pull := func(minutes, newPages, commitSec int) {
		lag, done := 0, 0
		for s := 0; s < minutes*60; s += commitSec {
			inFlight <- done // ask for the next page only when there is room
			<-inFlight       // write the page and wait for the commit
			done++
		}
		lag = minutes*newPages - done
		if lag < 0 {
			lag = 0
		}
		fmt.Printf("commit %d s: %d pages in %d minutes, at most %d page in memory, lag %d pages\n", commitSec, done, minutes, cap(inFlight), lag)
	}
	pull(10, 5, 10)
	pull(10, 5, 60)
}
