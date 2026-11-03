// Tell and ask with Go actors: a goroutine reading a channel is the actor,
// the channel is its mailbox. Replays the three tabs of 06-tell-and-ask.html.
package main

import (
	"errors"
	"fmt"
	"time"
)

type Msg any

// Actor is a goroutine that takes one message at a time from its mailbox.
type Actor struct {
	name    string
	mailbox chan Msg
}

func spawn(name string, handle func(self *Actor, m Msg)) *Actor {
	a := &Actor{name: name, mailbox: make(chan Msg, 100)}
	go func() {
		for m := range a.mailbox { // one message at a time
			handle(a, m)
		}
	}()
	return a
}

// Tell puts the message in the mailbox and returns at once.
func (a *Actor) Tell(m Msg) { a.mailbox <- m }

var errTimeout = errors.New("ask timed out")

// Ask is a tell with a temporary reply mailbox and a timeout.
func Ask(a *Actor, build func(replyTo chan Msg) Msg, timeout time.Duration) (Msg, error) {
	reply := make(chan Msg, 1) // the temporary reply actor
	a.Tell(build(reply))       // the message carries the reply address
	select {
	case r := <-reply:
		return r, nil // the future completes
	case <-time.After(timeout):
		return nil, errTimeout // the future fails; a late reply is dropped
	}
}

// Messages. Immutable values.
type Telemetry struct{ C float64 }
type ReadingStored struct{ C float64 }
type GetState struct{ ReplyTo chan Msg }
type State struct {
	Last  float64
	Count int
}
type Busy struct{ D time.Duration }
type GetLimit struct{ ReplyTo chan Msg }
type GetLast struct{ ReplyTo chan Msg }
type LimitIs struct{ C float64 }
type LastIs struct{ C float64 }
type Check struct{}

const askTimeout = 2 * time.Second

func main() {
	start := time.Now()
	at := func() string { return fmt.Sprintf("%4.1fs", time.Since(start).Seconds()) }

	// Tab 1: tell.
	rules := spawn("rules", func(self *Actor, m Msg) {
		if r, ok := m.(ReadingStored); ok {
			fmt.Printf("%s rules: ReadingStored %.1f C\n", at(), r.C)
		}
	})
	var st State
	device := spawn("device-42", func(self *Actor, m Msg) {
		switch v := m.(type) {
		case Telemetry:
			st.Last, st.Count = v.C, st.Count+1
			rules.Tell(ReadingStored{v.C})
		case GetState:
			v.ReplyTo <- st // a copy of the state, never the live state
		case Busy:
			time.Sleep(v.D)
		}
	})
	device.Tell(Telemetry{21.5})
	fmt.Printf("%s api: told TelemetryReceived 21.5 C, returned at once\n", at())
	time.Sleep(50 * time.Millisecond)

	// Tab 2: ask at the edge.
	r, err := Ask(device, func(rt chan Msg) Msg { return GetState{rt} }, askTimeout)
	fmt.Printf("%s api: ask GetState -> %+v, err=%v (HTTP answered)\n", at(), r, err)

	for i := 0; i < 50; i++ { // 50 messages ahead of GetState, 50 ms each = 2.5 s
		device.Tell(Busy{50 * time.Millisecond})
	}
	r, err = Ask(device, func(rt chan Msg) Msg { return GetState{rt} }, askTimeout)
	fmt.Printf("%s api: ask GetState behind 50 messages -> %v, err=%v (error returned)\n", at(), r, err)
	time.Sleep(600 * time.Millisecond) // the late reply goes nowhere

	// Tab 3: an ask awaited inside a handler, in both directions: deadlock.
	var dev2, rules2 *Actor
	rules2 = spawn("rules", func(self *Actor, m Msg) {
		switch v := m.(type) {
		case Check:
			_, err := Ask(dev2, func(rt chan Msg) Msg { return GetLast{rt} }, askTimeout) // blocks rules
			fmt.Printf("%s rules: ask GetLast inside handler -> %v\n", at(), err)
		case GetLimit:
			v.ReplyTo <- LimitIs{30}
		}
	})
	dev2 = spawn("device-42", func(self *Actor, m Msg) {
		switch v := m.(type) {
		case Telemetry:
			_, err := Ask(rules2, func(rt chan Msg) Msg { return GetLimit{rt} }, askTimeout) // blocks device
			fmt.Printf("%s device: ask GetLimit inside handler -> %v\n", at(), err)
		case GetLast:
			v.ReplyTo <- LastIs{21.5}
		}
	})
	dev2.Tell(Telemetry{21.5})
	rules2.Tell(Check{})
	time.Sleep(askTimeout + 300*time.Millisecond)

	// The fix: tell with a reply-to, handle the answer as a message.
	done := make(chan struct{})
	var dev3 *Actor
	rules3 := spawn("rules", func(self *Actor, m Msg) {
		if v, ok := m.(GetLimit); ok {
			v.ReplyTo <- LimitIs{30}
		}
	})
	dev3 = spawn("device-42", func(self *Actor, m Msg) {
		switch v := m.(type) {
		case Telemetry:
			rules3.Tell(GetLimit{ReplyTo: self.mailbox}) // reply comes to my own mailbox
		case LimitIs:
			fmt.Printf("%s device: LimitIs %.0f C arrived as a message, handler never blocked\n", at(), v.C)
			close(done)
		}
	})
	dev3.Tell(Telemetry{21.5})
	<-done
}
