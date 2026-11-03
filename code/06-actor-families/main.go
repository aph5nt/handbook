// Two actor families. Classic: you build the tree, supervise and stop. A message to a
// stopped actor goes to dead letters. Virtual: an actor exists by its key, the first
// call activates it, idle deactivates it, and a failure goes back to the caller.
package main

import (
	"errors"
	"fmt"
)

// ---- classic: Akka.NET, Erlang ----

type Directive string

// Supervisor owns its children. decide is the domain rule, written in one place.
type Supervisor struct {
	children map[string]chan string
	dead     int
}

func decide(err string) Directive {
	switch err {
	case "connection broke":
		return "Restart"
	case "corrupt packet":
		return "Resume"
	}
	return "Escalate"
}

// child is a goroutine with a mailbox. It reports failures on exit and answers done.
func (s *Supervisor) spawn(name string, generation int, exit chan string) {
	mb := make(chan string, 4)
	s.children[name] = mb
	fmt.Printf("  %s: generation %d, connection open\n", name, generation)
	go func() {
		for m := range mb {
			if m == "connection broke" {
				exit <- m // the actor fails and tells its parent
				return
			}
			fmt.Printf("  %s: handled %s\n", name, m)
			exit <- ""
		}
	}()
}

func (s *Supervisor) tell(name, m string) {
	if mb, ok := s.children[name]; ok {
		mb <- m
		return
	}
	s.dead++ // nothing appears because a message names it
	fmt.Printf("  %q to %s: no actor, dead letters %d\n", m, name, s.dead)
}

// ---- virtual: Orleans ----

type PlayerState struct{ Score int }

type Grain struct {
	key     int
	state   PlayerState
	silo    int
	mailbox chan func(*Grain) // one call at a time
}

// Runtime holds the directory: key to activation. Callers never say where.
type Runtime struct {
	active  map[int]*Grain
	storage map[int]PlayerState
	silos   []int // next placement choices
}

func (rt *Runtime) activation(key int) *Grain {
	if g, ok := rt.active[key]; ok {
		return g
	}
	silo := rt.silos[0]
	rt.silos = rt.silos[1:]
	g := &Grain{key: key, state: rt.storage[key], silo: silo, mailbox: make(chan func(*Grain))}
	go func() {
		for call := range g.mailbox {
			call(g)
		}
	}()
	rt.active[key] = g
	fmt.Printf("  runtime: activate player %d on silo %d, load score %d\n", key, silo, g.state.Score)
	return g
}

func (rt *Runtime) deactivate(key int) {
	rt.storage[key] = rt.active[key].state
	fmt.Printf("  runtime: idle, save score %d, deactivate player %d on silo %d\n", rt.storage[key].Score, key, rt.active[key].silo)
	close(rt.active[key].mailbox)
	delete(rt.active, key)
}

func (rt *Runtime) call(key int, method string, fn func(*Grain) error) error {
	g := rt.activation(key) // GetGrain by key: no create, no reference to hold
	result := make(chan error)
	g.mailbox <- func(g *Grain) { result <- fn(g) }
	err := <-result // the exception travels back to the caller
	fmt.Printf("  player %d on silo %d: %s, err=%v\n", key, g.silo, method, err)
	return err
}

func main() {
	fmt.Println("== classic: you own the lifecycle")
	sup := &Supervisor{children: map[string]chan string{}}
	exit := make(chan string)
	sup.spawn("device-42", 1, exit)
	sup.tell("device-42", "Reading 21.5 C")
	<-exit
	sup.tell("device-42", "connection broke")
	reason := <-exit
	d := decide(reason)
	fmt.Printf("  supervisor: %q -> %s\n", reason, d)
	sup.spawn("device-42", 2, exit)
	fmt.Printf("  supervisor: %q -> %s\n", "corrupt packet", decide("corrupt packet"))
	close(sup.children["device-42"])
	delete(sup.children, "device-42") // your code stops it
	fmt.Println("  device-42: stopped by your code")
	sup.tell("device-42", "Reading 21.7 C")

	fmt.Println("\n== virtual: the runtime owns the lifecycle")
	rt := &Runtime{active: map[int]*Grain{}, storage: map[int]PlayerState{}, silos: []int{2, 1}}
	fmt.Printf("  GetGrain(7): active activations %d\n", len(rt.active))
	rt.call(7, "JoinMatch", func(g *Grain) error { return nil })
	rt.call(7, "AddScore 120", func(g *Grain) error { g.state.Score += 120; return nil })
	rt.deactivate(7)
	fmt.Printf("  active activations %d, stored score %d\n", len(rt.active), rt.storage[7].Score)
	rt.call(7, "GetStats", func(g *Grain) error { return nil })
	err := rt.call(7, "GetStats", func(g *Grain) error { return errors.New("stats store down") })
	fmt.Printf("  game server: no supervisor, the exception is mine: %v\n", err)
}
