// Saga orchestration versus choreography: order 501 for one phone. Payment and
// stock succeed, shipping refuses the address. Both runs end with a refund and
// a released phone. The difference is where the plan lives, and who can answer
// "where is order 501?".
package main

import "fmt"

// Shop is the state of the three services for one order.
type Shop struct {
	Payment, Stock, Shipping, Order string
}

// Step is one line of the orchestrator's plan.
type Step struct {
	Name string
	Do   func() bool
	Undo func()
}

// Row is the orchestrator's own record of one order.
type Row struct {
	Step  int
	State string
}

// Orchestrator holds the plan and one row per order.
type Orchestrator struct {
	Plan  []Step
	Table map[int]*Row
}

// Run walks the plan, and walks it backwards on a refusal.
func (o *Orchestrator) Run(id int) {
	row := &Row{Step: 1, State: "running"}
	o.Table[id] = row
	for i, st := range o.Plan {
		row.Step = i + 1
		if !st.Do() {
			row.State = fmt.Sprintf("failed at step %d", row.Step)
			for j := i - 1; j >= 0; j-- {
				o.Plan[j].Undo()
			}
			row.State = fmt.Sprintf("cancelled at step %d", row.Step)
			return
		}
	}
	row.State = "done"
}

// Bus delivers each event to everyone who listens for its name.
type Bus struct {
	subs map[string][]func(id int)
	all  []func(id int, event string)
}

// On registers a listener for one event name.
func (b *Bus) On(event string, f func(id int)) {
	b.subs[event] = append(b.subs[event], f)
}

// Publish calls the listeners. Nothing here knows the order of the steps.
func (b *Bus) Publish(id int, event string) {
	for _, f := range b.all {
		f(id, event)
	}
	for _, f := range b.subs[event] {
		f(id)
	}
}

// Wire is the whole flow, spread over each service's own listeners.
func Wire(b *Bus, s *Shop, addressOK bool) {
	b.On("OrderPlaced", func(id int) { s.Payment = "charged"; b.Publish(id, "PaymentTaken") })
	b.On("PaymentTaken", func(id int) { s.Stock = "reserved"; b.Publish(id, "StockReserved") })
	b.On("StockReserved", func(id int) {
		if addressOK {
			s.Shipping = "booked"
			b.Publish(id, "Shipped")
			return
		}
		s.Shipping = "refused"
		b.Publish(id, "ShippingFailed")
	})
	b.On("ShippingFailed", func(id int) { s.Stock = "released" })
	b.On("ShippingFailed", func(id int) { s.Payment = "refunded" })
	b.On("ShippingFailed", func(id int) { s.Order = "cancelled" })
}

// Track is the fix: a read model that records the last event per order.
func Track(b *Bus, rows map[int]string) {
	b.all = append(b.all, func(id int, event string) { rows[id] = event })
}

func main() {
	fmt.Println("== Orchestration")
	s := &Shop{}
	o := &Orchestrator{Table: map[int]*Row{}, Plan: []Step{
		{"charge", func() bool { s.Payment = "charged"; return true }, func() { s.Payment = "refunded" }},
		{"reserve", func() bool { s.Stock = "reserved"; return true }, func() { s.Stock = "released" }},
		{"book courier", func() bool { s.Shipping = "refused"; return false }, func() {}},
	}}
	o.Run(501)
	fmt.Printf("payment %s, stock %s, shipping %s\n", s.Payment, s.Stock, s.Shipping)
	fmt.Printf("where is order 501? one row: step %d, %s\n", o.Table[501].Step, o.Table[501].State)

	fmt.Println("== Choreography")
	s = &Shop{Order: "saved"}
	b := &Bus{subs: map[string][]func(int){}}
	rows := map[int]string{}
	Wire(b, s, false)
	Track(b, rows)
	b.Publish(501, "OrderPlaced")
	fmt.Printf("payment %s, stock %s, shipping %s, order %s\n", s.Payment, s.Stock, s.Shipping, s.Order)
	fmt.Printf("where is order 501? only with the tracker: last event %s\n", rows[501])
}
