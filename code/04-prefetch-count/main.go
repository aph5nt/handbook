// Prefetch count: how many unacked messages the broker sends one consumer.
//
// Tab 1 computes the throughput rule: at most prefetch messages per network
// round trip, and at most one per processing time. Tab 2 replays jobs dealt to
// workers, the way RabbitMQ does with basic.qos. The round trip and processing
// times are this example's. Deterministic.
package main

import "fmt"

// throughput is messages a second for one consumer.
func throughput(prefetch int, rttMs, workMs float64) float64 {
	byNetwork := float64(prefetch) / ((rttMs + workMs) / 1000)
	byWork := 1000 / workMs
	if byNetwork < byWork {
		return byNetwork
	}
	return byWork
}

// deal gives jobs to workers in connection order, up to each prefetch.
func deal(jobs, prefetch, workers int) (held []int, queue int) {
	held = make([]int, workers)
	queue = jobs
	for w := 0; w < workers && queue > 0; w++ {
		n := prefetch
		if n > queue {
			n = queue
		}
		held[w] = n
		queue -= n
	}
	return
}

func main() {
	fmt.Println("== Tab 1: tiny messages ==")
	const rtt, work = 1.0, 0.1 // milliseconds
	fmt.Printf("prefetch 1:   %.0f messages a second\n", throughput(1, rtt, work))
	fmt.Printf("prefetch 100: %.0f messages a second, at most 100 held\n", throughput(100, rtt, work))

	fmt.Println("== Tab 2: heavy jobs ==")
	held, queue := deal(150, 100, 3) // [100 50 0], queue 0
	fmt.Printf("prefetch 100: workers hold %v, queue %d\n", held, queue)
	done := 2              // worker 1 crashes on job 3
	back := held[0] - done // crash on job 3: 98 go back
	fmt.Printf("worker 1 crashes on job 3: %d jobs go back at once\n", back)
	held, queue = deal(150, 1, 3) // [1 1 1], queue 147
	fmt.Printf("prefetch 1: workers hold %v, queue %d\n", held, queue)
	back = held[0] // a crash returns 1 job
	fmt.Printf("worker 1 crashes: %d job goes back, workers 2 and 3 keep going\n", back)
}
