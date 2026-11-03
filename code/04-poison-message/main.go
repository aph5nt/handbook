// Poison message: tell a permanent error from a transient one.
//
// Plain Go values stand in for the decoder, the price database and the
// DLQ. Errors carry a kind: a parse error is permanent, a timeout is
// transient. Time is a counter in seconds, so nothing sleeps. Deterministic.
package main

import (
	"errors"
	"fmt"
)

var (
	ErrTimeout = errors.New("price lookup: timeout")
	ErrParse   = errors.New("amount: expected number, got text")
)

// permanent says whether retrying can ever help.
func permanent(err error) bool { return errors.Is(err, ErrParse) }

// dlq parks a message with its error, so the stream keeps flowing.
func dlq(err error) {
	fmt.Println("  parked in the DLQ with its headers:", err)
}

func main() {
	fmt.Println("== Tab 1: tell them apart ==")
	dbUpAt := 2 // the price database is back at second 2
	m3 := func(t int) error {
		if t < dbUpAt {
			return ErrTimeout
		}
		return nil
	}
	m7 := func(int) error { return ErrParse }
	fmt.Printf("t=0 m3: %v, m7: %v\n", m3(0), m7(0))
	fmt.Printf("t=2 retry m3: err=%v, m7: %v (same error again)\n", m3(2), m7(2))
	fmt.Println("m7 parked in the DLQ with its error and headers, the stream keeps flowing")

	fmt.Println("== Tab 2: classify ==")
	handle := func(name string, try func(int) error) {
		t := 0
		for attempt := 1; ; attempt++ {
			err := try(t)
			if err == nil {
				fmt.Printf("%s: ok after %d attempt(s), t=%ds\n", name, attempt, t)
				return
			}
			if permanent(err) {
				fmt.Printf("%s: permanent (%v): DLQ on attempt %d, t=%ds\n", name, err, attempt, t)
				dlq(err) // on the first failure, with its headers
				return
			}
			t += 1 << (attempt - 1) // transient: wait 1, 2, 4 ... s
		}
	}
	handle("m7", m7)
	handle("m3", m3)

	// Wrong guess 1: every error transient, 5 retries with backoff.
	waited := 0
	for a := 0; a < 5; a++ {
		waited += 1 << a
	}
	fmt.Printf("wrong guess 1: m7 retried 5 times over %d s before parking\n", waited)

	// Wrong guess 2: every error poison, the database down for 2 minutes.
	const perMinute = 20000
	fmt.Printf("wrong guess 2: database down 2 min at %d msgs/min: %d healthy messages parked\n", perMinute, 2*perMinute)

	// Circuit breaker: 5 transient failures in a row pause consumption.
	fails, paused := 0, false
	for i := 0; i < 8; i++ {
		if paused {
			break
		}
		fails++ // every message fails with ErrTimeout while the DB is down
		if fails >= 5 {
			paused = true
		}
	}
	fmt.Printf("breaker: open after %d timeouts in a row, consumption paused, 0 parked\n", fails)
}
