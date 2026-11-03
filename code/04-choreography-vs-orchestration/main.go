// Choreography versus orchestration of a top-up: Wallet, Units, Receipt,
// Ledger, Notify.
//
// Plain function calls and slices stand in for a broker: publish walks the
// subscribers, and the orchestrator's command is a function call that returns
// an event. Deterministic.
package main

import "fmt"

// Choreography: each service knows only which event it hears and publishes.
var subscribers = map[string][]string{
	"TopUpPaid":    {"Units"},
	"UnitsGranted": {"Receipt"},
	"ReceiptSent":  {"Ledger"},
	"LedgerPosted": {"Notify"},
}

var publishes = map[string]string{
	"Units": "UnitsGranted", "Receipt": "ReceiptSent", "Ledger": "LedgerPosted", "Notify": "",
}

// publish delivers an event, stopping if a service is down. It returns who ran.
func publish(event string, down map[string]bool, ran *[]string, hops *int, maxHops int) {
	*hops++
	if *hops > maxHops {
		return
	}
	for _, service := range subscribers[event] {
		if down[service] {
			return // crashed before it published: the chain stops here, silently
		}
		*ran = append(*ran, service)
		if next := publishes[service]; next != "" {
			publish(next, down, ran, hops, maxHops)
		}
	}
}

// Orchestration: one record per top-up, with the state and the time.
type record struct{ id, state, since string }

// call sends a command and returns false if no answer comes back.
func call(service string, down map[string]bool) bool { return !down[service] }

func main() {
	fmt.Println("== Tab 1: choreography ==")
	var ran []string
	hops := 0
	publish("TopUpPaid", nil, &ran, &hops, 20)
	fmt.Println("T-1042 ran:", ran)

	ran, hops = nil, 0
	publish("TopUpPaid", map[string]bool{"Units": true}, &ran, &hops, 20)
	fmt.Printf("T-1043 with Units down ran: %v, and no service knows it stopped\n", ran)

	// The cycle: Wallet hears LedgerPosted and publishes TopUpPaid by mistake.
	subscribers["LedgerPosted"] = []string{"Notify", "Wallet"}
	publishes["Wallet"] = "TopUpPaid"
	ran, hops = nil, 0
	publish("TopUpPaid", nil, &ran, &hops, 12)
	grants := 0
	for _, s := range ran {
		if s == "Units" {
			grants++
		}
	}
	fmt.Printf("cycle, cut off after 12 events: Units granted %d times\n", grants)

	fmt.Println("== Tab 2: orchestration ==")
	r := record{id: "T-1043", state: "Started", since: "10:00"}
	down := map[string]bool{"Units": true}
	if call("Wallet", down) {
		r.state, r.since = "Paid", "10:02"
	}
	if !call("Units", down) {
		fmt.Printf("support looks up %s: %s, waiting on Units since %s\n", r.id, r.state, r.since)
	}
	down["Units"] = false // Units restarts
	granted := map[string]bool{}
	for try := 0; try < 2; try++ { // a repeat of the same command
		granted[r.id] = true // idempotent: keyed by the top-up id
	}
	fmt.Printf("GrantUnits sent twice, grants: %d\n", len(granted))
	for _, step := range []string{"Receipt", "Ledger", "Notify"} {
		call(step, down)
		r.state = step + " done"
	}
	fmt.Printf("%s ends in state: %s\n", r.id, r.state)
}
