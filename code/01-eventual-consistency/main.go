// Eventual consistency: the wallet commits at t0, the subscription at t1.
// Between them two true facts disagree: design for that window.
package main

import "fmt"

// World is the two aggregates and the events still travelling.
type World struct {
	Balance  int
	Renewed  bool
	InFlight []string
	Stopped  bool // the subscription consumer is down
}

// TopUp is t0: the wallet commits alone and the window opens.
func (w *World) TopUp(n int) {
	w.Balance += n
	w.InFlight = append(w.InFlight, "WalletToppedUp")
}

// Deliver is t1: the subscription handles the event in its own transaction.
func (w *World) Deliver() {
	if w.Stopped {
		return // events pile up in the broker
	}
	for range w.InFlight {
		w.Renewed = true
	}
	w.InFlight = nil
}

// Lag is what you measure and alert on.
func (w *World) Lag() int { return len(w.InFlight) }

// Page shows the account. justToppedUp comes from the command's reply.
func (w *World) Page(justToppedUp bool) string {
	switch {
	case w.Renewed:
		return fmt.Sprintf("%d USD, renewed", w.Balance)
	case justToppedUp:
		return fmt.Sprintf("%d USD, renewal in progress", w.Balance)
	}
	return fmt.Sprintf("%d USD, unpaid", w.Balance)
}

// ReminderOnRead is the failure: an irreversible act on a stale read.
func (w *World) ReminderOnRead() string {
	if !w.Renewed {
		return "email sent: plan cancelled"
	}
	return "no email"
}

// ReminderOnLag is the fix: never act while events are still travelling.
func (w *World) ReminderOnLag() string {
	if w.Lag() > 0 {
		return "wait: the window is open"
	}
	return w.ReminderOnRead()
}

func main() {
	w := &World{}
	fmt.Println("start:", w.Page(false))
	w.TopUp(20)
	fmt.Println("t0:", w.Page(false), "- lag", w.Lag())
	fmt.Println("page knows:", w.Page(true))
	fmt.Println("job on a read:", w.ReminderOnRead())
	fmt.Println("job on the lag:", w.ReminderOnLag())
	w.Deliver()
	fmt.Println("t1:", w.Page(false), "- lag", w.Lag())
	x := &World{Stopped: true}
	x.TopUp(20)
	x.Deliver()
	fmt.Println("consumer stopped:", x.Page(false), "- lag", x.Lag())
}
