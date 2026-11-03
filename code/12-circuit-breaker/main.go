// Circuit breaker: the story of the lesson 12-circuit-breaker.html.
// An MCP proxy queries graph lane B through a breaker. Five failures inside a
// 10-second window trip it open. Open fails fast. After 30 s one probe goes
// through: a failure opens it again, a success closes it.
package main

import (
	"errors"
	"fmt"
	"time"
)

type State string

const (
	Closed   State = "closed"
	Open     State = "open"
	HalfOpen State = "half-open"

	threshold = 5
	window    = 10 * time.Second
	openFor   = 30 * time.Second
)

type Breaker struct {
	State    State
	failures []time.Duration // times of recent failures
	openedAt time.Duration
}

var ErrOpen = errors.New("lane unavailable (fast error)")

// Call runs one query at time now. laneOK says whether lane B answers.
func (b *Breaker) Call(now time.Duration, laneOK bool) error {
	if b.State == Open {
		if now-b.openedAt < openFor {
			return ErrOpen // no call reaches lane B
		}
		b.State = HalfOpen // 30 s passed: let exactly one probe through
	}
	if laneOK {
		b.State, b.failures = Closed, nil // success closes and resets the count
		return nil
	}
	if b.State == HalfOpen {
		b.State, b.openedAt = Open, now // the probe failed: open again
		return errors.New("probe failed")
	}
	b.failures = append(b.failures, now)
	for len(b.failures) > 0 && now-b.failures[0] > window {
		b.failures = b.failures[1:] // drop failures older than the window
	}
	if len(b.failures) >= threshold {
		b.State, b.openedAt = Open, now // trip
	}
	return errors.New("timeout after 2 s")
}

func main() {
	b := &Breaker{State: Closed}
	s := time.Second
	plan := []struct {
		at time.Duration
		ok bool
	}{{0, true}, {2 * s, false}, {4 * s, false}, {6 * s, false}, {8 * s, false}, {10 * s, false},
		{12 * s, false}, {40 * s, false}, {50 * s, true}, {71 * s, true}, {72 * s, true}}
	for _, p := range plan {
		err := b.Call(p.at, p.ok)
		res := "answer"
		if err != nil {
			res = err.Error()
		}
		fmt.Printf("t=%2.0fs lane ok=%-5v -> %-30s state %s, failures in window %d\n",
			p.at.Seconds(), p.ok, res, b.State, len(b.failures))
	}

	fmt.Println("== No breaker: the retry storm")
	agents := 100
	fmt.Printf("normal: %d calls, load 40%%\n", agents)
	fmt.Printf("every call fails, all retry at the same instant: %d calls\n", 2*agents)
	fmt.Printf("3 layers x 3 tries: one request becomes %d calls\n", 3*3*3)
	fmt.Printf("with a breaker, once open: 1 probe per %.0f s reaches lane B\n", openFor.Seconds())
}
