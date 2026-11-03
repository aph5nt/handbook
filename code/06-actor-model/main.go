// The actor model (Hewitt, 1973): on a message an actor may send, create, and choose
// its next behaviour. Nothing else. The second part shows what a fourth move costs.
package main

import (
	"fmt"
	"sync"
)

type Msg interface{}

type Register struct {
	ID      int
	ReplyTo chan string
}
type DeviceIs struct {
	ID   int
	Addr *Actor // an address travels inside a message
}
type Change struct {
	ID    int
	Field string
}
type Read struct {
	ID    int
	Reply chan string
}

// Actor: an address and a mailbox. The behaviour is the function for the next message.
type Actor struct {
	name    string
	mailbox chan Msg
}

type Behaviour func(self *Actor, m Msg) Behaviour

// spawn is move 2. The loop applies move 3: each handler returns the next behaviour.
func spawn(name string, b Behaviour) *Actor {
	a := &Actor{name: name, mailbox: make(chan Msg, 4)}
	go func() {
		for m := range a.mailbox {
			b = b(a, m)
		}
	}()
	return a
}

// send is move 1. It may fail to deliver: the model promises nothing about arrival.
func send(to *Actor, m Msg) bool {
	select {
	case to.mailbox <- m:
		return true
	default:
		return false // a full mailbox stands in for any lost message
	}
}

func idle(self *Actor, m Msg) Behaviour { return idle }

func registry(rules *Actor, known map[int]bool) Behaviour {
	return func(self *Actor, m Msg) Behaviour {
		r := m.(Register)
		if known[r.ID] {
			r.ReplyTo <- "already exists" // no device actor is created
			return registry(rules, known)
		}
		dev := spawn(fmt.Sprintf("device-%d", r.ID), idle) // move 2
		send(rules, DeviceIs{r.ID, dev})                   // move 1
		next := map[int]bool{r.ID: true}                   // move 3: a new behaviour
		for k := range known {
			next[k] = true
		}
		r.ReplyTo <- "created " + dev.name
		return registry(rules, next)
	}
}

func main() {
	fmt.Println("== the three moves")
	learned := make(chan string, 1)
	rules := spawn("rules", func(self *Actor, m Msg) Behaviour {
		d := m.(DeviceIs)
		learned <- "rules knows " + d.Addr.name
		return idle
	})
	reg := spawn("registry", registry(rules, map[int]bool{}))
	reply := make(chan string, 1)
	send(reg, Register{42, reply})
	fmt.Println("registry:", <-reply)
	fmt.Println(<-learned)
	send(reg, Register{42, reply})
	fmt.Println("registry, same message:", <-reply)

	busy := &Actor{name: "busy", mailbox: make(chan Msg)} // nobody reads it
	fmt.Println("send to a full mailbox delivered:", send(busy, Reading{}))

	fmt.Println("\n== the shortcut: a static cache, two writers")
	type Entry struct{ Registered, Online bool }
	cache := map[int]Entry{42: {}} // shared memory: not one of the three moves
	var mu sync.Mutex              // keeps the map safe, and does not save the update
	var bothRead sync.WaitGroup
	bothRead.Add(2)
	deviceWrote := make(chan struct{})
	finished := make(chan struct{})
	go func() { // the registry, on one thread
		mu.Lock()
		e := cache[42] // reads the old entry
		mu.Unlock()
		bothRead.Done()
		bothRead.Wait()
		<-deviceWrote
		e.Registered = true
		mu.Lock()
		cache[42] = e // writes it back: this one wins
		mu.Unlock()
		close(finished)
	}()
	go func() { // device 42, on another thread
		mu.Lock()
		e := cache[42] // reads the same old entry
		mu.Unlock()
		bothRead.Done()
		bothRead.Wait()
		e.Online = true
		mu.Lock()
		cache[42] = e
		mu.Unlock()
		close(deviceWrote)
	}()
	<-finished
	fmt.Printf("cache: %+v (online is lost)\n", cache[42])

	fmt.Println("\n== the fix: one owner, one message at a time")
	list := map[int]map[string]bool{}
	var owner func(self *Actor, m Msg) Behaviour
	owner = func(self *Actor, m Msg) Behaviour {
		switch v := m.(type) {
		case Change:
			if list[v.ID] == nil {
				list[v.ID] = map[string]bool{}
			}
			list[v.ID][v.Field] = true
		case Read:
			v.Reply <- fmt.Sprintf("42: registered=%v online=%v", list[v.ID]["registered"], list[v.ID]["online"])
		}
		return owner
	}
	own := spawn("owner", owner)
	send(own, Change{42, "registered"}) // from the registry
	send(own, Change{42, "online"})     // from device 42
	r := make(chan string, 1)
	send(own, Read{42, r})
	fmt.Println("owner:", <-r)
}

// Reading is any message.
type Reading struct{}
