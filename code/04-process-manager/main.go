// Process manager: one saved state machine per top-up.
//
// A map stands in for the durable store, and a function call for an event
// arriving from the broker. Time is a plain counter of minutes, so the timers
// are deterministic. The timer and retry limit are this example's.
package main

import "fmt"

// next says, for a state and an event, the new state and the command to send.
var next = map[[2]string][2]string{
	{"Started", "TopUpPaid"}:         {"Paid", "GrantUnits"},
	{"Paid", "UnitsGranted"}:         {"Units granted", "SendReceipt"},
	{"Units granted", "ReceiptSent"}: {"Done", ""},
	{"Refunding", "PaymentRefunded"}: {"Cancelled, refunded", ""},
}

type process struct {
	state   string
	retries int
	history []string
}

// send publishes a command. An empty command means the case is finished.
func send(cmd string) {
	if cmd != "" {
		fmt.Println("  send", cmd)
	}
}

type manager struct{ cases map[string]*process }

func (m *manager) start(id string) {
	m.cases[id] = &process{state: "Started", history: []string{"Started"}}
	fmt.Printf("%s: Started\n", id)
	send("ConfirmPayment")
}

// on handles one event. The correlation id picks the record; the table decides.
func (m *manager) on(id, event string) {
	if id == "" {
		fmt.Printf("%s with no correlation id: parked, a person looks at it\n", event)
		return
	}
	p := m.cases[id]
	move, ok := next[[2]string{p.state, event}]
	if !ok {
		fmt.Printf("%s: %s while %s: ignored, a duplicate\n", id, event, p.state)
		return
	}
	p.state = move[0]
	p.retries = 0
	p.history = append(p.history, p.state)
	fmt.Printf("%s: %s -> %s\n", id, event, p.state)
	send(move[1])
}

// timer fires 5 minutes after a command with no answer.
func (m *manager) timer(id string, minute int) {
	p := m.cases[id]
	if p.retries == 0 {
		p.retries++
		fmt.Printf("%s: minute %d, no answer\n", id, minute)
		send("GrantUnits") // same idempotency key
		return
	}
	p.state = "Refunding"
	p.history = append(p.history, p.state)
	fmt.Printf("%s: minute %d, second timer, -> Refunding\n", id, minute)
	send("RefundPayment")
}

func main() {
	m := &manager{cases: map[string]*process{}}

	fmt.Println("== Tab 1: happy path ==")
	m.start("T-1043")
	m.on("T-1043", "TopUpPaid")
	m.on("T-1043", "UnitsGranted")
	m.on("T-1043", "ReceiptSent")
	fmt.Println("T-1043 history:", m.cases["T-1043"].history)

	fmt.Println("== Tab 2: timeout ==")
	m.start("T-1044")
	m.on("T-1044", "TopUpPaid")
	m.timer("T-1044", 5)
	m.timer("T-1044", 10)
	m.on("T-1044", "PaymentRefunded")
	fmt.Println("T-1044 history:", m.cases["T-1044"].history)

	fmt.Println("== Tab 3: two cases ==")
	m.start("T-1045")
	m.start("T-1046")
	m.on("T-1045", "TopUpPaid")
	m.on("T-1046", "TopUpPaid")
	m.on("T-1045", "UnitsGranted")
	m.on("", "UnitsGranted")
	m.on("T-1045", "UnitsGranted")
	m.on("T-1046", "UnitsGranted")
	m.on("T-1045", "ReceiptSent")
	m.on("T-1046", "ReceiptSent")
	fmt.Println("T-1045:", m.cases["T-1045"].state, "T-1046:", m.cases["T-1046"].state)
}
