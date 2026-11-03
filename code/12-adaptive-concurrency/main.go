// Adaptive concurrency limits: the story of 12-adaptive-concurrency.html.
// An MCP proxy learns how many queries to send to a graph lane at once. AIMD:
// add 5 while latency stays near the baseline, halve when it rises past
// 1.5 times the baseline, refuse at once above the limit.
package main

import "fmt"

// Graph answers in 20 ms up to its capacity, then each extra query in flight
// adds 3 ms of queueing.
func Latency(inFlight, capacity int) int {
	if inFlight <= capacity {
		return 20
	}
	return 20 + (inFlight-capacity)*3
}

type Limiter struct {
	Limit    int
	Baseline int // best latency seen: no queueing
}

// Tick runs every few seconds with the latency just measured.
func (l *Limiter) Tick(latency int) {
	if latency < l.Baseline {
		l.Baseline = latency
	}
	if latency*2 > l.Baseline*3 { // past 1.5 x baseline: the graph queues
		l.Limit /= 2 // multiplicative decrease
		return
	}
	l.Limit += 5 // additive increase
}

// Admit refuses at once above the limit, so the query never reaches the graph.
func (l *Limiter) Admit(inFlight int) bool { return inFlight < l.Limit }

func run(l *Limiter, capacity, ticks int) {
	for t := 0; t < ticks; t++ {
		lat := Latency(l.Limit, capacity)
		fmt.Printf("%d(%dms) ", l.Limit, lat)
		l.Tick(lat)
	}
	fmt.Println()
}

func main() {
	l := &Limiter{Limit: 20, Baseline: 20}
	fmt.Println("== capacity 55: limit(latency) each tick")
	run(l, 55, 18)
	fmt.Printf("query number %d when the limit is %d: admitted = %v\n", l.Limit+1, l.Limit, l.Admit(l.Limit))
	fmt.Println("== a sync starts, capacity 25")
	l.Limit = 55
	run(l, 25, 10)

	fmt.Println("== A fixed limit")
	fmt.Printf("fixed 100 on a busy day (capacity 50): %d ms per query, 50 queued inside the graph\n", Latency(100, 50))
	queued, rounds := 50, 1
	for Latency(50+queued, 50) <= 5000 {
		queued += 50 // a timed-out query keeps running; its retry joins behind it
		rounds++
	}
	fmt.Printf("retries keep 100 in flight: after %d rounds %d queued inside, %d ms: past the 5 s timeout\n", rounds, queued, Latency(50+queued, 50))
	fmt.Printf("fixed 20 on a quiet day (capacity 120): %d ms, but %d slots left idle\n", Latency(20, 120), 120-20)
}
