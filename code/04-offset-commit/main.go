// Offset commit: auto versus manual, with a crash while charging order 105.
//
// A slice stands in for one Kafka partition (offsets 100..109), an int for
// the committed offset stored in Kafka, and a map for the payments table.
// With auto-commit, Kafka's client commits the position of the last poll
// on a 5-second timer, whatever the code has finished. Deterministic.
package main

import "fmt"

type Consumer struct {
	committed int          // what Kafka has stored for the group
	charges   map[int]bool // payments table, keyed by order id
	auto      bool
}

// poll returns the next batch from the committed offset.
func poll(from int) []int {
	var out []int
	for o := from; o < 110; o++ {
		out = append(out, o)
	}
	return out
}

// runBatch charges each order. crashAt < 0 means no crash.
// timerAfter is how many charges finish before the auto-commit timer fires.
func (c *Consumer) runBatch(crashAt, timerAfter int) (done int, crashed bool) {
	batch := poll(c.committed)
	position := batch[len(batch)-1] + 1 // the next offset after this poll
	for i, o := range batch {
		if c.auto && i == timerAfter {
			c.committed = position // timer: commits the poll position, 110
		}
		if o == crashAt {
			return done, true
		}
		if c.charges[o] { // the idempotency key exists: skip
			continue
		}
		c.charges[o] = true // charge and key in one transaction
		done++
	}
	if !c.auto {
		c.committed = position // manual: commit after the work
	}
	return done, false
}

func main() {
	fmt.Println("== Tab 1: auto-commit ==")
	c := &Consumer{committed: 100, charges: map[int]bool{}, auto: true}
	done, crashed := c.runBatch(105, 5)
	fmt.Printf("polled 100..109, charged %d, timer committed %d, crashed=%v at 105\n", done, c.committed, crashed)
	fmt.Printf("restart at %d: nothing to read\n", c.committed)
	lost := 0
	for o := 100; o < 110; o++ {
		if !c.charges[o] {
			lost++
		}
	}
	fmt.Printf("orders never charged: %d (105..109)\n", lost)

	fmt.Println("== Tab 2: manual commit ==")
	m := &Consumer{committed: 100, charges: map[int]bool{}}
	done, crashed = m.runBatch(105, -1)
	fmt.Printf("charged %d, committed still %d, crashed=%v at 105\n", done, m.committed, crashed)
	done, _ = m.runBatch(-1, -1)
	fmt.Printf("restart at 100: 5 skipped by key, %d charged, committed %d\n", done, m.committed)
	fmt.Printf("every order charged once: %v (%d charges)\n", len(m.charges) == 10, len(m.charges))
}
