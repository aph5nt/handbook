// Policy: one event in, one command out, no memory.
// Process manager: stored state, correlation, and an ending for every path.
package main

import "fmt"

// Policy has no memory.
func Policy(event string) string {
	if event == "WalletToppedUp" {
		return "SendReceipt" // one command out
	}
	return ""
}

type Renewal struct { // one instance, stored between events

	ID, Customer string
	State        string
	TimerDay     int
}

var Instances = map[string]*Renewal{} // correlation: found by customer id

// On handles one event on one day and returns the command to send next.
func (r *Renewal) On(event string, day int) string {
	switch {
	case event == "RenewalDue":
		r.State = "started"
		return "ChargeWallet(20)"
	case event == "ChargeFailed":
		r.State = "waiting for funds"
		r.TimerDay = day + 7
		return ""
	case event == "WalletToppedUp" && r.State == "waiting for funds":
		r.State = "charging"
		return "ChargeWallet(20)"
	case event == "WalletCharged":
		r.TimerDay = 0
		r.State = "done"
		return "ExtendPlan"
	case event == "TimerFired" && r.TimerDay != 0 && day >= r.TimerDay:
		r.State = "cancelled"
		return "CancelPlan"
	}
	return ""
}

// Handlers is the failure: no state, nobody connects the two.
func Handlers(event string, day int) string {
	if event == "ChargeFailed" {
		return fmt.Sprintf("schedule cancel on day %d", day+7)
	}
	return "SendReceipt" // the top-up handler: knows nothing of the cancel
}

func main() {
	fmt.Println("policy:", Policy("WalletToppedUp"))
	r := &Renewal{ID: "r-12", Customer: "c-17"}
	Instances[r.Customer] = r
	fmt.Println("day 0 RenewalDue ->", r.On("RenewalDue", 0), "|", r.State)
	r.On("ChargeFailed", 0)
	fmt.Println("day 0 ChargeFailed:", r.State, "- timer day", r.TimerDay)
	me := Instances["c-17"] // correlation, from the event's customer id
	fmt.Println("day 2 WalletToppedUp ->", me.On("WalletToppedUp", 2), "|", me.State)
	fmt.Println("day 2 WalletCharged ->", me.On("WalletCharged", 2), "|", me.State)
	t := &Renewal{ID: "r-13", Customer: "c-18"}
	t.On("RenewalDue", 0)
	t.On("ChargeFailed", 0)
	fmt.Println("day 7 TimerFired ->", t.On("TimerFired", 7), "|", t.State)
	fmt.Println("no manager, day 0:", Handlers("ChargeFailed", 0))
	fmt.Println("no manager, day 2:", Handlers("WalletToppedUp", 2), "- day 7 still cancels")
}
