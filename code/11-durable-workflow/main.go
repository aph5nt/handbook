// Command temporal replays the durable workflow lesson: a refund written as
// plain code, a worker crash during a three-day sleep, replay from history
// with no second refund, a code change that breaks replay, and the version
// check that fixes it. This is a small model of the Temporal idea.
package main

import "fmt"

type Event struct{ Name, Result string }

type Bank struct{ Refunds int }

var (
	errStop = fmt.Errorf("stop")
	bank    = &Bank{}
)

type Run struct {
	History []Event // lives on the server, survives any worker
	next    int
	Real    int // real activity calls by this worker
	Fired   bool
}

func (r *Run) Activity(name string, do func() string) string {
	if r.next < len(r.History) {
		e := r.History[r.next]
		r.next++
		if e.Name != name {
			panic(fmt.Errorf("non-determinism: code says %q, history says %q", name, e.Name))
		}
		return e.Result
	}
	res := do()
	r.Real++
	r.History = append(r.History, Event{name, res})
	r.next++
	return res
}

func (r *Run) Sleep(name string) {
	r.Activity(name, func() string { return "timer started" })
	if !r.Fired {
		panic(errStop)
	}
}

func (r *Run) Version(change string) int {
	if r.next < len(r.History) && r.History[r.next].Name != "version "+change {
		return 1
	}
	r.Activity("version "+change, func() string { return "2" })
	return 2
}

// Execute is any worker running the code from line 1 with the history.
func Execute(r *Run, wf func(*Run)) (err error) {
	r.next, r.Real = 0, 0
	defer func() {
		if p := recover(); p != nil {
			err = p.(error)
		}
	}()
	wf(r)
	return nil
}

// Now is the safe clock: the first run records it, a replay reads it back.
func (r *Run) Now() string {
	return r.Activity("now", func() string { return "10:00" })
}

func Refund(r *Run) {
	// BUG to avoid here: time.Now() or rand gives a new value on replay.
	r.Activity("refund card", func() string { bank.Refunds++; return "R-881" })
	r.Sleep("sleep 3 days")
	r.Activity("check bank", func() string { return "confirmed" })
	r.Activity("email Ana", func() string { return "sent" })
}

func main() {
	fmt.Println("crash tab")
	run := &Run{}
	err := Execute(run, Refund)
	fmt.Printf("step 2-3: worker 1: history %v, stops at the timer: %v\n", run.History, err)
	fmt.Println("step 4: worker 1 crashes on day 2; history on the server has", len(run.History), "events")
	run.Fired = true
	err = Execute(run, Refund)
	fmt.Printf("step 5-7: worker 2 replays: real calls %d, bank refunds %d, err %v\n", run.Real, bank.Refunds, err)
	fmt.Printf("  history %v\n", run.History)

	fmt.Println("nondet tab")
	old := &Run{History: []Event{{"refund card", "R-881"}, {"sleep 3 days", "timer started"}}, Fired: true}
	changed := func(r *Run) {
		r.Activity("refund card", func() string { return "R-881" })
		r.Activity("fraud check", func() string { return "clean" })
		r.Sleep("sleep 3 days")
		r.Activity("check bank", func() string { return "confirmed" })
		r.Activity("email Ana", func() string { return "sent" })
	}
	fmt.Println("step 3: replay with new code:", Execute(old, changed))
	versioned := func(r *Run) {
		r.Activity("refund card", func() string { return "R-881" })
		if r.Version("fraud-check") == 2 {
			r.Activity("fraud check", func() string { return "clean" })
		}
		r.Sleep("sleep 3 days")
		r.Activity("check bank", func() string { return "confirmed" })
		r.Activity("email Ana", func() string { return "sent" })
	}
	old = &Run{History: []Event{{"refund card", "R-881"}, {"sleep 3 days", "timer started"}}, Fired: true}
	err = Execute(old, versioned)
	fmt.Printf("step 4: old run with version check: err %v, fraud check skipped, %d events\n", err, len(old.History))
	fresh := &Run{Fired: true}
	Execute(fresh, versioned)
	fmt.Printf("  new run: %d events, includes the fraud check\n", len(fresh.History))
	fmt.Println("step 5: time.Now() in workflow code would differ on replay; the engine returns the recorded time")
}
