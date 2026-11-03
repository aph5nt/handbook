// Restart backoff: a poisoned row fails on every start. Plain restart turns it into a
// storm. A backoff supervisor fails slowly, then stops and tells a person.
// Waits are scaled: 1 ms in the program stands for 1 s in the story.
package main

import (
	"errors"
	"fmt"
	"math/rand"
	"time"
)

type Msg interface{}

type Start struct{}
type Failed struct{ Err error }
type Healthy struct{}

var errCorrupt = errors.New("row 42: corrupt")

// Database row 42.
var rowOK = false
var queries = 0

func preStart() error {
	queries++
	if !rowOK {
		return errCorrupt
	}
	return nil
}

// device is the actor: its loop takes Start and runs PreStart.
func device(mailbox, parent chan Msg) {
	go func() {
		for range mailbox {
			if err := preStart(); err != nil {
				parent <- Failed{err}
			} else {
				parent <- Healthy{}
			}
		}
	}()
}

// nextWait doubles from 1 s up to 30 s, and adds jitter up to 20 percent.
func nextWait(failures int, r *rand.Rand) (base, jittered float64) {
	base = 1.0
	for i := 1; i < failures && base < 30; i++ {
		base *= 2
	}
	if base > 30 {
		base = 30
	}
	return base, base * (1 + 0.2*r.Float64())
}

func backoff(r *rand.Rand) {
	in, sup := make(chan Msg, 4), make(chan Msg, 4)
	device(in, sup)
	failures, t := 0, 0.0
	in <- Start{}
	for m := range sup {
		switch m.(type) {
		case Failed:
			failures++
			if failures >= 4 { // the limit: 4 tries
				fmt.Printf("  t=%4.1f s: failure %d, limit reached: stop device 42, alert. Queries %d\n", t, failures, queries)
				return
			}
			base, wait := nextWait(failures, r)
			fmt.Printf("  t=%4.1f s: failure %d, wait %.0f s + jitter = %.2f s\n", t, failures, base, wait)
			t += wait
			time.AfterFunc(time.Duration(wait*float64(time.Millisecond)), func() { in <- Start{} })
		case Healthy:
			fmt.Println("  device 42: running")
			return
		}
	}
}

func main() {
	fmt.Println("== plain restart: no wait, no limit")
	const queryMs = 5 // an assumed cost of one query plus its stack trace
	fmt.Printf("  one try costs %d ms, so %d failures a second, %d a minute, and no alert fires\n", queryMs, 1000/queryMs, 60*1000/queryMs)

	fmt.Println("\n== backoff: 1 s, doubled, at most 30 s, jitter, 4 tries")
	r := rand.New(rand.NewSource(42))
	backoff(r)
	fmt.Printf("  a plain restart would have made about %d queries in the same 8 s\n", 8*1000/queryMs)
	base := []string{}
	for f := 1; f <= 7; f++ {
		b, _ := nextWait(f, r)
		base = append(base, fmt.Sprintf("%.0f", b))
	}
	fmt.Println("  the wait with a higher limit:", base, "seconds, capped at 30")

	fmt.Println("\n== a person fixes row 42")
	rowOK = true
	backoff(r)

	fmt.Println("\n== 1,000 actors retry after the same 1 s wait: busiest 10 ms")
	noJitter, withJitter := map[int]int{}, map[int]int{}
	for i := 0; i < 1000; i++ {
		noJitter[100]++
		_, w := nextWait(1, r)
		withJitter[int(w*100)]++
	}
	fmt.Printf("  no jitter: %d queries, with jitter: %d queries\n", peak(noJitter), peak(withJitter))

	fmt.Println("\n== a poisoned Kafka record, offsets 9000 to 9005, 9001 is bad")
	consume(false)
	consume(true)
}

func peak(m map[int]int) int {
	best := 0
	for _, n := range m {
		if n > best {
			best = n
		}
	}
	return best
}

// consume reads partition 3. With no dead-letter topic it stops after 6 tries, standing
// in for "for ever", and shows the lag behind the bad record.
func consume(deadLetter bool) {
	const end = 9006
	mailbox := make(chan int, 1)
	done := make(chan struct{})
	offset, tries := 9000, 0
	var parked []int
	go func() {
		defer close(done)
		for o := range mailbox {
			if o != 9001 {
				offset, tries = o+1, 0 // commit
				if offset == end {
					return
				}
				mailbox <- offset
				continue
			}
			tries++
			if deadLetter && tries >= 3 { // after a few tries: park it, commit, move on
				parked = append(parked, o)
				offset, tries = o+1, 0
				fmt.Printf("  record %d: parked in the dead-letter topic after 3 tries\n", o)
				mailbox <- offset
				continue
			}
			if !deadLetter && tries == 6 {
				return
			}
			mailbox <- o // not committed: the same record is read again
		}
	}()
	mailbox <- offset
	<-done
	if deadLetter {
		fmt.Printf("  dead-letter topic: offset %d committed, lag %d, parked %v\n", offset, end-offset, parked)
	} else {
		fmt.Printf("  no dead-letter topic: %d tries on 9001, offset stuck at %d, lag %d and growing\n", tries, offset, end-offset)
	}
}
