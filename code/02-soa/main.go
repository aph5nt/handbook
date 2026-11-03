// Replays the SOA lesson: accounts, loans and cards linked by a smart
// enterprise service bus with one canonical Customer model, then the same
// three changes with smart endpoints and a dumb pipe.
package main

import "fmt"

var systems = []string{"Accounts", "Loans", "Cards"}

// Change is one piece of work a team wants to ship.
type Change struct {
	team, what string
	touches    string // "model", "rule" or "route"
}

// Pipe is what links the systems: a smart bus or a dumb broker.
type Pipe struct {
	name      string
	routes    map[string]string   // message type -> system (the bus decides)
	canonical map[string]bool     // fields of the one company Customer model
	mappedBy  []string            // systems mapped into and out of that model
	rules     map[string][]string // business rule -> the checks the pipe runs
}

// isBus is the gateway test: can the pipe team change a business outcome?
func (p *Pipe) isBus() bool { return len(p.rules) > 0 || len(p.canonical) > 0 }

// changeField adds a field to the canonical model and lists who must retest.
func (p *Pipe) changeField(team, field string) (retest []string, mappings int) {
	p.canonical[field] = true
	for _, s := range p.mappedBy {
		if s != team {
			retest = append(retest, s)
		}
	}
	return retest, len(p.mappedBy) // every mapping in the bus changes too
}

// waitsOnPipeTeam: a change that touches a route, the model or a rule held by the pipe.
func (p *Pipe) waitsOnPipeTeam(c Change) bool {
	switch c.touches {
	case "model":
		return len(p.canonical) > 0
	case "rule":
		return len(p.rules) > 0
	case "route":
		return len(p.routes) > 0
	}
	return false
}

// Service is a smart endpoint with its own model.
type Service struct {
	name  string
	model map[string]bool // its own idea of a customer
	needs []string        // fields it reads from other services' events
}

// consume reads only the fields it needs. A new optional field breaks nothing.
func (s *Service) consume(event map[string]string) (ignored int, broken bool) {
	for _, f := range s.needs {
		if _, ok := event[f]; !ok {
			broken = true
		}
	}
	return len(event) - len(s.needs), broken
}

// approveLoan lives in Loans: ask the two APIs, decide here.
func approveLoan(accountOK, cardOK bool) bool { return accountOK && cardOK }

func queue(p *Pipe, changes []Change) int {
	n := 0
	for _, c := range changes {
		if p.waitsOnPipeTeam(c) {
			n++
		}
	}
	return n
}

func main() {
	changes := []Change{
		{"Cards", "add field loyalty_tier to Customer", "model"},
		{"Loans", "change the loan approval flow", "rule"},
		{"Accounts", "send statements to a new system", "route"},
	}

	fmt.Println("tab 1: SOA with a smart bus")
	bus := &Pipe{
		name:      "ESB",
		routes:    map[string]string{"AccountOpened": "Loans", "CardIssued": "Accounts"},
		canonical: map[string]bool{"id": true, "name": true, "address": true},
		mappedBy:  systems,
		rules:     map[string][]string{},
	}
	fmt.Printf("  step 1: AccountOpened goes to %s, the bus decides\n", bus.routes["AccountOpened"])
	fmt.Printf("  step 2: canonical Customer fields: %d, mapped by %d systems\n", len(bus.canonical), len(bus.mappedBy))
	bus.rules["approveLoan"] = []string{"account check", "card check"}
	fmt.Printf("  step 3: rule approveLoan in the bus runs %v. is the pipe a bus? %v\n", bus.rules["approveLoan"], bus.isBus())
	retest, maps := bus.changeField("Cards", "loyalty_tier")
	fmt.Printf("  step 4: Cards adds loyalty_tier. retest: %v, bus mappings to change: %d\n", retest, maps)
	fmt.Printf("  step 5: changes waiting on the bus team: %d of %d\n", queue(bus, changes), len(changes))

	fmt.Println("tab 2: smart endpoints, dumb pipes")
	broker := &Pipe{name: "Message broker"}
	fmt.Printf("  step 1: broker routes %d, rules %d, model fields %d\n", len(broker.routes), len(broker.rules), len(broker.canonical))
	acc := &Service{"Accounts", map[string]bool{"id": true, "iban": true}, []string{"id"}}
	lns := &Service{"Loans", map[string]bool{"id": true, "income": true}, []string{"id", "limit"}}
	fmt.Printf("  step 2: Accounts model fields %d, Loans model fields %d, shared model fields 0\n", len(acc.model), len(lns.model))
	ev := map[string]string{"id": "c-17", "limit": "5000", "loyalty_tier": "gold"} // CardIssued v2
	ia, ba := acc.consume(ev)
	il, bl := lns.consume(ev)
	fmt.Printf("  step 3: Cards adds optional loyalty_tier. ignored: Accounts %d, Loans %d. broken: %v %v\n", ia, il, ba, bl)
	fmt.Printf("  step 4: Loans asks Accounts and Cards, decides: approve = %v\n", approveLoan(true, true))
	fmt.Printf("  changes waiting on the pipe team: %d of %d\n", queue(broker, changes), len(changes))
	gw := &Pipe{name: "API gateway", rules: map[string][]string{"freeShipping": {"order over 100"}}}
	fmt.Printf("  step 5: broker is a bus? %v. gateway with rule freeShipping is a bus? %v\n", broker.isBus(), gw.isBus())
}
