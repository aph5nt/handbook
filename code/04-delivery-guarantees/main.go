// Delivery guarantees: at-most-once, at-least-once and effectively-once,
// replayed with a crash at the same point in each.
//
// A Go struct stands in for the broker: a delivered message stays
// "unacked" and is delivered again after the consumer restarts. The
// payments table and the processed-keys table are maps in one "database",
// and tx() makes a write to both all-or-nothing. Deterministic.
package main

import "fmt"

type Broker struct{ queue []int }

func (b *Broker) Deliver() (int, bool) {
	if len(b.queue) == 0 {
		return 0, false
	}
	return b.queue[0], true
}
func (b *Broker) Ack() { b.queue = b.queue[1:] } // deleted for good

type DB struct {
	charges   map[int]int // order -> times charged
	processed map[int]bool
}

// tx applies both writes or neither, like one database transaction.
func (d *DB) tx(order int) {
	d.charges[order]++
	d.processed[order] = true
}

// crash simulates the consumer dying at a point in its handler.
type crash struct{}

func run(mode string) map[int]int {
	b := &Broker{queue: []int{501}}
	d := &DB{charges: map[int]int{}, processed: map[int]bool{}}
	handle := func(order int, crashNow bool) (crashed bool) {
		defer func() {
			if recover() != nil {
				crashed = true
			}
		}()
		switch mode {
		case "at-most-once":
			b.Ack() // ack first: the broker deletes it
			if crashNow {
				panic(crash{}) // dies before the charge
			}
			d.charges[order]++
		case "at-least-once":
			d.charges[order]++ // work first
			if crashNow {
				panic(crash{}) // dies before the ack
			}
			b.Ack()
		case "effectively-once":
			if d.processed[order] { // key found: skip the work
				b.Ack()
				return
			}
			d.tx(order) // charge and key commit together
			if crashNow {
				panic(crash{})
			}
			b.Ack()
		}
		return
	}
	first := true
	for {
		order, ok := b.Deliver()
		if !ok {
			break
		}
		if handle(order, first) {
			fmt.Printf("  %s: crashed while handling order %d, restarting\n", mode, order)
		}
		first = false
	}
	return d.charges
}

func main() {
	for _, m := range []string{"at-most-once", "at-least-once", "effectively-once"} {
		c := run(m)
		fmt.Printf("%s: order 501 charged %d time(s)\n", m, c[501])
	}
}
