// Become and stash: the current handler is the state. Early messages wait in a stash.
// Time is scaled: 1 ms in the program stands for 1 s in the story.
package main

import (
	"fmt"
	"time"
)

type Msg interface{}

type SetInterval struct{ S int }
type GetState struct{}
type Connected struct{}
type ConnectTimeout struct{}

type Device struct {
	mailbox  chan Msg
	pending  []Msg // unstashed messages, taken before the mailbox
	stash    []Msg
	capacity int
	behavior func(d *Device, m Msg) // become swaps this
	interval int
	dead     int // messages dropped to dead letters
	unstash  bool
	done     chan struct{}
}

func (d *Device) become(b func(*Device, Msg)) { d.behavior = b }

func (d *Device) stashMsg(m Msg) {
	if d.capacity > 0 && len(d.stash) >= d.capacity {
		d.dead++ // stash full: dead letters
		return
	}
	d.stash = append(d.stash, m)
}

func (d *Device) unstashAll() {
	d.pending = append(d.stash, d.pending...) // back to the front, in order
	d.stash = nil
}

func (d *Device) run() {
	for {
		var m Msg
		if len(d.pending) > 0 {
			m, d.pending = d.pending[0], d.pending[1:]
		} else {
			var ok bool
			if m, ok = <-d.mailbox; !ok {
				return
			}
		}
		d.behavior(d, m) // one handler, the current one
	}
}

func connecting(d *Device, m Msg) {
	switch m.(type) {
	case Connected:
		d.become(online)
		fmt.Println("device 42: Connected -> become Online")
		if d.unstash {
			fmt.Printf("device 42: unstash %d messages to the front\n", len(d.stash))
			d.unstashAll()
		}
	case ConnectTimeout:
		d.dead += len(d.stash)
		fmt.Printf("device 42: 30 s timeout -> become Failed, %d stashed to dead letters, supervisor retries later\n", len(d.stash))
		d.stash = nil
		d.become(failed)
		close(d.done)
	default:
		d.stashMsg(m) // not now: keep it, in order
	}
}

func online(d *Device, m Msg) {
	switch v := m.(type) {
	case SetInterval:
		d.interval = v.S
		fmt.Printf("device 42: Online applies SetInterval %d s on the thermostat\n", v.S)
	case GetState:
		fmt.Printf("device 42: Online answers GetState: interval %d s\n", d.interval)
		close(d.done)
	}
}

func failed(d *Device, m Msg) { d.dead++ }

func spawn(capacity int, unstash bool) *Device {
	d := &Device{mailbox: make(chan Msg, 10000), capacity: capacity, unstash: unstash,
		behavior: connecting, interval: 60, done: make(chan struct{})}
	go d.run()
	return d
}

func main() {
	fmt.Println("== stash, become, unstash")
	d := spawn(0, true)
	d.mailbox <- SetInterval{30}
	d.mailbox <- GetState{}
	time.Sleep(5 * time.Millisecond)
	fmt.Printf("device 42: Connecting stashed %d messages\n", len(d.stash))
	d.mailbox <- Connected{}
	<-d.done

	fmt.Println("\n== thermostat unplugged, no limit, no timeout")
	d = spawn(0, true)
	for i := 0; i < 5000; i++ {
		d.mailbox <- SetInterval{30}
	}
	time.Sleep(20 * time.Millisecond)
	fmt.Printf("device 42: stash holds %d messages and keeps growing\n", len(d.stash))
	fmt.Println("10,000 unplugged devices x 5,000 messages = 50,000,000 messages held in memory")

	fmt.Println("\n== connects, but Online forgets unstash")
	d = spawn(0, false)
	d.mailbox <- SetInterval{30}
	d.mailbox <- Connected{}
	time.Sleep(5 * time.Millisecond)
	fmt.Printf("device 42: Online, interval still %d s, stuck in the stash: %d\n", d.interval, len(d.stash))

	fmt.Println("\n== the fix: capacity 100 and a 30 s timeout")
	d = spawn(100, true)
	go func() { time.Sleep(30 * time.Millisecond); d.mailbox <- ConnectTimeout{} }()
	for i := 0; i < 5000; i++ {
		d.mailbox <- SetInterval{30}
	}
	<-d.done
	fmt.Printf("device 42: stash empty, %d messages went to dead letters, memory stays flat\n", d.dead)
}
