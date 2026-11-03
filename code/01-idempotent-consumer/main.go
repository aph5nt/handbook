// Idempotent consumer: delivery is at least once, so the effect must be exactly
// once. The event id and the effect commit in one transaction.
package main

import "fmt"

var months = []string{"November", "December", "January"}

// Plan is the subscription: an index into months, plus the months already paid.
type Plan struct {
	Until     int
	PaidBy    map[string]bool // the set form: month paid by this top-up
	Processed map[string]bool // processed_events, with a unique key on the id
}

// Handle inserts the id and applies the effect in one transaction.
func Handle(p *Plan, id string) string {
	if p.Processed[id] { // the insert hits the unique key
		return "duplicate: skipped"
	}
	p.Processed[id] = true
	p.Until++ // add one month
	return "applied"
}

// HandleNoDedup is the failure: every delivery adds a month.
func HandleNoDedup(p *Plan, id string) string {
	p.Until++
	return "applied again"
}

// HandleSet makes the effect idempotent itself: set, not add.
func HandleSet(p *Plan, id string) {
	if !p.PaidBy[id] {
		p.PaidBy[id] = true
		p.Until = len(p.PaidBy)
	}
}

// Kafka resends from the last committed offset.
type Kafka struct{ Committed int }

func (k *Kafka) Commit(offset int) { k.Committed = offset }

func newPlan() *Plan { return &Plan{PaidBy: map[string]bool{}, Processed: map[string]bool{}} }

func main() {
	p := newPlan()
	k := &Kafka{Committed: 41}
	fmt.Println("e-81, first:", Handle(p, "e-81"), "- paid until", months[p.Until])
	fmt.Println("crash before commit: Kafka resends offset", k.Committed+1)
	fmt.Println("e-81, second:", Handle(p, "e-81"), "- paid until", months[p.Until])
	k.Commit(42)
	fmt.Println("offset committed:", k.Committed)
	s := newPlan()
	HandleSet(s, "e-81")
	HandleSet(s, "e-81")
	fmt.Println("set, twice: paid until", months[s.Until])
	n := newPlan()
	fmt.Println("no dedup:", HandleNoDedup(n, "e-81"), HandleNoDedup(n, "e-81"), "- paid until", months[n.Until])
}
