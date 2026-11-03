// Timeouts and retries: the story of the lesson 12-timeouts-and-retries.html.
// Five indexer workers call a blockchain node that blips. Fixed waits make
// the retries arrive as a wall. Exponential backoff with jitter spreads them.
// A deadline caps the retries, and only idempotent calls may be repeated.
package main

import (
	"errors"
	"fmt"
	"time"
)

const (
	base     = 200 * time.Millisecond // first backoff wait
	maxTries = 4                      // the first call plus three retries
)

// Node models the blip: down until 0.5 s, then it admits only the first two
// calls between 0.5 s and 1.5 s while it warms up, then it is healthy.
type Node struct {
	admitted int
	counter  int
}

func (n *Node) Call(at time.Duration) error {
	switch {
	case at < 500*time.Millisecond:
		return errors.New("stalled")
	case at < 1500*time.Millisecond:
		if n.admitted >= 2 {
			return errors.New("warming up: over the limit")
		}
		n.admitted++
	}
	return nil
}

// Backoff is the wait before retry number attempt (0, 1, 2): the base
// doubled each time, plus a jitter draw between 0 and that wait.
func Backoff(attempt int, jitter time.Duration) time.Duration {
	wait := base << attempt // 200 ms, 400 ms, 800 ms
	return wait + jitter    // jitter is drawn from [0, wait)
}

// Retry calls the node until it answers, the tries run out, or the deadline
// leaves no time. Non-idempotent calls are never repeated.
func Retry(n *Node, deadline time.Duration, idempotent bool, jitter []time.Duration) (time.Duration, []time.Duration, error) {
	var now time.Duration
	var calls []time.Duration
	for attempt := 0; attempt < maxTries; attempt++ {
		if now >= deadline {
			return now, calls, errors.New("deadline spent: give up")
		}
		calls = append(calls, now)
		if err := n.Call(now); err == nil {
			return now, calls, nil
		}
		if !idempotent {
			return now, calls, errors.New("not idempotent: do not repeat")
		}
		if attempt == maxTries-1 {
			break
		}
		now += Backoff(attempt, jitter[attempt])
	}
	return now, calls, errors.New("gave up after max tries")
}

func ms(d time.Duration) string { return fmt.Sprintf("%.2f s", d.Seconds()) }

func main() {
	fmt.Println("== Fixed wait: the retry storm")
	// Every call hangs 2 s on the stalled node, then every worker waits the same 200 ms.
	t := time.Duration(0)
	for wave := 0; wave < 3; wave++ {
		fmt.Printf("wave %d: all 5 calls at %s\n", wave, ms(t))
		t += 2*time.Second + base
	}
	fmt.Println("3 layers x 3 tries each =", 3*3*3, "calls for one request")

	fmt.Println("== Exponential backoff with jitter")
	// The jitter draws of the lesson, one row per worker, one column per retry.
	jit := [][]time.Duration{
		{20 * time.Millisecond, 130 * time.Millisecond, 0},
		{110 * time.Millisecond, 240 * time.Millisecond, 350 * time.Millisecond},
		{60 * time.Millisecond, 140 * time.Millisecond, 0},
		{180 * time.Millisecond, 270 * time.Millisecond, 450 * time.Millisecond},
		{150 * time.Millisecond, 250 * time.Millisecond, 200 * time.Millisecond},
	}
	deadline := []time.Duration{10 * time.Second, 10 * time.Second, 10 * time.Second, 10 * time.Second, 1500 * time.Millisecond}
	n := &Node{}
	// Only retry 2 lands in the warm-up window, so replay the workers in the
	// order their retry 2 arrives: 0.75, 0.80, 0.95, 1.00, 1.05 s.
	results := make([]string, 5)
	order := []int{0, 2, 1, 4, 3}
	for _, w := range order {
		at, calls, err := Retry(n, deadline[w], true, jit[w])
		s := ""
		for _, c := range calls {
			s += ms(c) + " "
		}
		if err != nil {
			results[w] = fmt.Sprintf("worker %d: calls at %s-> %v (next try would be %s)", w+1, s, err, ms(at))
		} else {
			results[w] = fmt.Sprintf("worker %d: calls at %s-> answered at %s", w+1, s, ms(at))
		}
	}
	for _, r := range results {
		fmt.Println(r)
	}
	// Worker 4 adds 1 to a counter. Its call at 1.05 s was applied, but the reply was lost.
	n2 := &Node{}
	n2.counter++ // applied at 1.05 s, the reply never came back
	n2.counter++ // the retry at 2.30 s applies it again
	fmt.Println("worker 4 counter +1, retried after a lost reply: counter =", n2.counter, "(should be 1)")
}
