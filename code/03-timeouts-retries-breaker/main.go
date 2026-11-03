// Timeouts, retries, circuit breaker and bulkhead. An API calls a graph
// database (one query now takes 40 s) and a warehouse (healthy). The run
// replays the lesson on a simulated clock: a shared pool starves, then a
// timeout, a bulkhead and a breaker keep the wallet endpoints alive, then
// the retry storm and its fix.
package main

import (
	"fmt"
	"math/rand"
	"time"
)

// Pool is a fixed number of threads. A call holds one until it returns.
type Pool struct {
	name       string
	size, busy int
}

// Acquire takes a free thread, or reports that none is free.
func (p *Pool) Acquire() bool {
	if p.busy == p.size {
		return false // every thread is held by a waiting call
	}
	p.busy++
	return true
}

// Release gives the thread back.
func (p *Pool) Release() { p.busy-- }

// Breaker states: Closed lets calls through, Open fails fast, HalfOpen lets one trial through.
type Breaker struct {
	state     string
	fails     int
	threshold int           // failures in a row before it opens
	openedAt  time.Duration // simulated time the breaker opened
	coolDown  time.Duration
}

// Allow decides if a call may leave the API at time now.
func (b *Breaker) Allow(now time.Duration) bool {
	if b.state == "Open" && now-b.openedAt >= b.coolDown {
		b.state = "HalfOpen" // the cool-down ended: let one trial through
	}
	return b.state != "Open"
}

// Record counts the result of a call that was allowed.
func (b *Breaker) Record(ok bool, now time.Duration) {
	if ok {
		b.state, b.fails = "Closed", 0 // a success closes and resets
		return
	}
	b.fails++
	if b.state == "HalfOpen" || b.fails >= b.threshold {
		b.state, b.openedAt = "Open", now // stop calling the sick service
	}
}

// CallGraph runs one graph call with a deadline. The thread is held for
// the shorter of the query time and the timeout.
func CallGraph(p *Pool, b *Breaker, query, timeout, now time.Duration) string {
	if !b.Allow(now) {
		return "fail fast: graph unavailable" // no call is made
	}
	if !p.Acquire() {
		return "refused: no free thread"
	}
	defer p.Release()
	if query > timeout {
		b.Record(false, now)
		return "timed out after " + timeout.String()
	}
	b.Record(true, now)
	return "ok"
}

// CallsReachingGraph: every one of n layers tries each call it gets this many times.
func CallsReachingGraph(layers, tries int) int {
	calls := 1
	for i := 0; i < layers; i++ {
		calls *= tries
	}
	return calls
}

// Budget allows a retry only while retries stay at most 10 % of all calls.
type Budget struct{ calls, retries int }

// MayRetry spends one retry if the budget allows it.
func (b *Budget) MayRetry() bool {
	if float64(b.retries+1) > 0.10*float64(b.calls) {
		return false // over budget: return the failure at once
	}
	b.retries++
	return true
}

// Backoff waits 100, 200, 400 ms for attempts 1, 2, 3, plus up to 50 % random jitter.
func Backoff(attempt int, jitter bool, r *rand.Rand) time.Duration {
	d := 100 * time.Millisecond << (attempt - 1)
	if jitter {
		d += time.Duration(r.Int63n(int64(d / 2)))
	}
	return d
}

func main() {
	fmt.Println("== No limits: one shared pool of 8 threads, no timeout")
	shared := &Pool{name: "shared", size: 8}
	for i := 0; i < 8; i++ {
		shared.Acquire() // each money flow request waits 40 s on the graph
	}
	fmt.Printf("8 graph calls waiting 40 s: %d of %d threads busy\n", shared.busy, shared.size)
	fmt.Printf("wallet request needs a thread: free %v: the whole API is down\n", shared.Acquire())

	fmt.Println("== Timeout 2 s, graph pool 4, warehouse pool 4, breaker opens after 5")
	graph, wh := &Pool{name: "graph", size: 4}, &Pool{name: "warehouse", size: 4}
	br := &Breaker{state: "Closed", threshold: 5, coolDown: 30 * time.Second}
	now := time.Duration(0)
	for i := 1; i <= 5; i++ {
		r := CallGraph(graph, br, 40*time.Second, 2*time.Second, now)
		fmt.Printf("graph call %d: %s, failures %d, breaker %s\n", i, r, br.fails, br.state)
		now += 2 * time.Second
	}
	fmt.Printf("money flow while open: %s\n", CallGraph(graph, br, 40*time.Second, 2*time.Second, now))
	fmt.Printf("wallet call through its own pool: free %v\n", wh.Acquire())
	now = br.openedAt + 30*time.Second
	fmt.Printf("cool-down over, trial call (graph recovered): %s, breaker %s\n", CallGraph(graph, br, 50*time.Millisecond, 2*time.Second, now), br.state)
	br.state, br.openedAt = "Open", now
	now += 30 * time.Second
	fmt.Printf("replay: trial call fails: %s, breaker %s\n", CallGraph(graph, br, 40*time.Second, 2*time.Second, now), br.state)

	fmt.Println("== Retry storm")
	for layers := 1; layers <= 5; layers++ {
		fmt.Printf("%d layers of 3 tries: %d calls\n", layers, CallsReachingGraph(layers, 3))
	}
	r := rand.New(rand.NewSource(7))
	fmt.Printf("no jitter, all clients retry at %v, %v, %v\n", Backoff(1, false, r), Backoff(2, false, r), Backoff(3, false, r))
	fmt.Printf("with jitter, one client waits %v, %v, %v\n", Backoff(1, true, r), Backoff(2, true, r), Backoff(3, true, r))
	bud := &Budget{calls: 1000}
	allowed := 0
	for i := 0; i < 1000; i++ {
		if bud.MayRetry() {
			allowed++
		}
	}
	fmt.Printf("retry budget, 1,000 failed calls: %d retries allowed\n", allowed)
	fmt.Printf("fix, only the client library tries 3 times: %d calls reach the graph\n", CallsReachingGraph(1, 3))
}
