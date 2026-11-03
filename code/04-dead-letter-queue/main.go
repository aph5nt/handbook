// Retries, poison messages and the dead-letter queue (DLQ).
//
// Go slices stand in for the main topic, the retry topic and the DLQ. A
// clock in seconds drives the retry delays (1 s, 2 s, 4 s), so the run is
// exact and needs no sleeps. m3 carries its amount as text and fails to
// decode every time. Deterministic.
package main

import (
	"errors"
	"fmt"
	"strconv"
)

type Msg struct {
	ID, Amount string
	Attempts   int
	DueAt      float64
}

const maxAttempts = 4

func decode(m Msg, acceptText bool) error {
	if _, err := strconv.Atoi(m.Amount); err != nil && !acceptText {
		return errors.New("amount: expected number, got text")
	}
	return nil
}

func main() {
	fmt.Println("== Tab 1: retry topic, then the DLQ ==")
	main := []Msg{{ID: "m1", Amount: "5"}, {ID: "m2", Amount: "7"}, {ID: "m3", Amount: "five"},
		{ID: "m4", Amount: "1"}, {ID: "m5", Amount: "2"}, {ID: "m6", Amount: "3"}}
	var retry, dlq []Msg
	var decoded []string
	now := 0.0
	handle := func(m Msg) {
		if err := decode(m, false); err != nil {
			m.Attempts++
			if m.Attempts >= maxAttempts {
				fmt.Printf("t=%gs %s attempt %d failed: to the DLQ with %q\n", now, m.ID, m.Attempts, err.Error())
				dlq = append(dlq, m)
				return
			}
			delay := float64(int(1) << (m.Attempts - 1)) // 1, 2, 4 s
			m.DueAt = now + delay
			fmt.Printf("t=%gs %s attempt %d failed: retry topic, due in %gs\n", now, m.ID, m.Attempts, delay)
			retry = append(retry, m)
			return
		}
		decoded = append(decoded, m.ID)
	}
	for _, m := range main { // the main topic never waits for m3
		handle(m)
	}
	for len(retry) > 0 {
		m := retry[0]
		retry = retry[1:]
		now = m.DueAt
		handle(m)
	}
	fmt.Printf("decoded %v, DLQ holds %d\n", decoded, len(dlq))
	// Fix and replay: a decoder that accepts text amounts.
	for _, m := range dlq {
		if decode(m, true) == nil {
			decoded = append(decoded, m.ID+" (replayed)")
		}
	}
	fmt.Printf("after the fix and replay: %v\n", decoded)

	fmt.Println("== Tab 2: no limit, retry in place ==")
	stuck := 0
	for t := 0.0; t < 60; t += 0.2 { // one minute of instant retries
		if decode(main[2], false) != nil {
			stuck++ // retried at once, in place
		}
	}
	fmt.Printf("one minute: %d failed attempts on m3, m4..m6 decoded: 0\n", stuck)
}
