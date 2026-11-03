// Splitter, router and aggregator for one block of transactions.
//
// Buffered channels stand in for the broker's topics: one for each decoder,
// and one default channel for types nobody handles. Time is a plain counter of
// minutes, and the 5 minute timeout is this example's. Deterministic.
package main

import (
	"errors"
	"fmt"
)

type part struct {
	block, n, of int // the block number is the correlation id
	kind         string
}

// split turns one block into one message per transaction.
func split(block int, kinds []string) []part {
	parts := make([]part, len(kinds))
	for i, k := range kinds {
		parts[i] = part{block, i + 1, len(kinds), k}
	}
	return parts
}

type topics struct{ dex, transfer, unrouted chan part }

// route reads the type and picks the channel. Nothing is ever dropped.
func (t topics) route(p part) string {
	switch p.kind {
	case "swap":
		t.dex <- p
		return "dex"
	case "transfer":
		t.transfer <- p
		return "transfer"
	}
	t.unrouted <- p
	return "unrouted"
}

// broken says the transfer decoder fails, as in tab 2.
var broken bool

var errParked = errors.New("parked in the DLQ")

// decoder: keeps block and n, the correlation and the part
func decode(p part) (part, error) {
	if p.kind == "transfer" && broken {
		return p, errParked // parked in the DLQ: no result
	}
	return p, nil
}

// publish sends an event about one block.
func publish(event string, block int) {
	fmt.Printf("publish %s %d\n", event, block)
}

type group struct {
	got, of, opened int
	have            map[int]bool
}

type aggregator struct {
	timeout int
	open    map[int]*group // in production: a durable store, keyed by block
}

// add stores a result. A full group is published and forgotten.
func (a *aggregator) add(p part, minute int) {
	g, ok := a.open[p.block]
	if !ok {
		g = &group{of: p.of, opened: minute, have: map[int]bool{}}
		a.open[p.block] = g
	}
	if g.have[p.n] {
		return // a repeat of the same part counts once
	}
	g.have[p.n] = true
	g.got++
	if g.got == g.of {
		publish("BlockDecoded", p.block)
		fmt.Printf("minute %d: %d of %d, group forgotten\n", minute, g.got, g.of)
		delete(a.open, p.block)
	}
}

// tick completes a group that has waited longer than the timeout.
func (a *aggregator) tick(minute int) {
	for block, g := range a.open {
		if a.timeout > 0 && minute-g.opened >= a.timeout {
			missing := []int{}
			for n := 1; n <= g.of; n++ {
				if !g.have[n] {
					missing = append(missing, n)
				}
			}
			publish("BlockIncomplete", block) // with the missing parts
			fmt.Printf("minute %d: missing part %v, alert\n", minute, missing)
			delete(a.open, block)
		}
	}
}

func main() {
	t := topics{make(chan part, 9), make(chan part, 9), make(chan part, 9)}

	fmt.Println("== Tab 1: block 9,506 ==")
	a := &aggregator{timeout: 5, open: map[int]*group{}}
	for _, p := range split(9506, []string{"swap", "transfer", "swap"}) {
		fmt.Printf("part %d of %d, %s -> %s\n", p.n, p.of, p.kind, t.route(p))
	}
	for len(t.dex)+len(t.transfer) > 0 {
		select {
		case p := <-t.dex:
			a.add(p, 0)
		case p := <-t.transfer:
			a.add(p, 0)
		}
	}
	fmt.Println("unknown type ->", t.route(part{9506, 1, 1, "blob"}), "channel, held:", len(t.unrouted))

	fmt.Println("== Tab 2: block 9,507 loses part 2 ==")
	broken = true
	for _, timeout := range []int{0, 5} {
		a = &aggregator{timeout: timeout, open: map[int]*group{}}
		for _, p := range split(9507, []string{"swap", "transfer", "swap"}) {
			r, err := decode(p) // the transfer decoder parks part 2
			if err != nil {
				fmt.Printf("part %d: %v\n", p.n, err)
				continue
			}
			a.add(r, 0)
		}
		for minute := 1; minute <= 120; minute++ {
			a.tick(minute)
		}
		if g, ok := a.open[9507]; ok {
			fmt.Printf("no timeout: minute 120, group still open, %d of %d\n", g.got, g.of)
		}
	}
}
