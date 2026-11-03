// Replays the choreography versus orchestration lesson. Choreography: each
// service reacts to the event before it, and a bug in Stock leaves order 4411
// stuck with nobody noticing until a timeout watcher looks. Orchestration: one
// order process calls each step and refunds when Stock is out of phones.
package main

import (
	"errors"
	"fmt"
)

// ---------- choreography: a bus and subscribers, no owner

type Bus struct {
	subs map[string][]func(order int)
	log  []string
}

func (b *Bus) subscribe(event string, h func(order int)) {
	b.subs[event] = append(b.subs[event], h)
}

func (b *Bus) publish(event string, order int) {
	b.log = append(b.log, fmt.Sprintf("%s(%d)", event, order))
	for _, h := range b.subs[event] { // nobody owns the flow
		h(order)
	}
}

// status is what each order looks like from the outside.
var status = map[int]string{}
var placedAt = map[int]int{} // minute the order was placed

func wire(b *Bus, stockBug bool) {
	b.subscribe("OrderPlaced", func(o int) { status[o] = "paid"; b.publish("Paid", o) })
	b.subscribe("Paid", func(o int) {
		status[o] = "reserved"
		if stockBug {
			status[o] = "paid" // the bug: the reservation fails silently
			return             // and Reserved is never published
		}
		b.publish("Reserved", o)
	})
	b.subscribe("Reserved", func(o int) { b.publish("Shipped", o) })
	b.subscribe("Shipped", func(o int) { status[o] = "done" })
}

// watch is the fix for choreography: a timeout watcher per order.
func watch(now, limit int) []int {
	var stuck []int
	for o, s := range status {
		if s != "done" && now-placedAt[o] > limit {
			stuck = append(stuck, o)
		}
	}
	return stuck
}

// ---------- orchestration: one process owns the sequence

type Step struct {
	name, undoName string
	do             func(order int) error
	undo           func(order int)
}

func run(steps []Step, order int, trace *[]string) error {
	var done []Step
	for _, s := range steps {
		if err := s.do(order); err != nil { // call and wait for the reply
			*trace = append(*trace, s.name+": "+err.Error())
			for i := len(done) - 1; i >= 0; i-- { // compensation, newest first
				done[i].undo(order)
				*trace = append(*trace, done[i].undoName)
			}
			return err
		}
		*trace = append(*trace, s.name+": ok")
		done = append(done, s)
	}
	return nil
}

var phones = 1 // Stock's own data. The rule that reads it stays in Stock.

func reserve(order int) error {
	if phones == 0 {
		return errors.New("out of stock")
	}
	phones--
	return nil
}

func main() {
	fmt.Println("tab 1: choreography")
	b := &Bus{subs: map[string][]func(int){}}
	wire(b, false)
	placedAt[4410] = 0
	b.publish("OrderPlaced", 4410) // Orders publishes and moves on
	fmt.Printf("  order 4410 events: %v, status %s\n", b.log, status[4410])

	b2 := &Bus{subs: map[string][]func(int){}}
	wire(b2, true)
	placedAt[4411] = 5
	b2.publish("OrderPlaced", 4411)
	fmt.Printf("  order 4411 events: %v, status %s\n", b2.log, status[4411])
	fmt.Printf("  watcher at minute 40, limit 30: stuck %v\n", watch(40, 30))

	fmt.Println("tab 2: orchestration")
	charged := map[int]bool{}
	steps := []Step{
		{"Payment charge", "Payment refund",
			func(o int) error { charged[o] = true; return nil },
			func(o int) { charged[o] = false }},
		{"Stock reserve", "Stock release", reserve, func(int) { phones++ }},
		{"Shipping book", "Shipping cancel", func(int) error { return nil }, func(int) {}},
	}
	for _, o := range []int{4410, 4412} {
		var trace []string
		err := run(steps, o, &trace)
		fmt.Printf("  order %d: %v, error: %v, still charged: %v\n", o, trace, err, charged[o])
	}
	fmt.Printf("  steps in the orchestrator: %d, stock rules in the orchestrator: 0\n", len(steps))
}
