// Location transparency: the same Tell, a local or a remote path underneath.
// Node 3 is simulated in-process: a goroutine reads bytes from a "network" channel.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

// GetState is a message. ReplyTo names the sender's address, Conn is the bug.
type GetState struct {
	ReplyTo string
	Conn    any `json:",omitempty"` // a live connection: not data
}

type State struct{ Device, Node string }

// Ref is all the sender holds: an address, never the actor.
type Ref struct {
	addr    string        // "local" or "node3:4053"
	mailbox chan GetState // used when local
	wire    chan []byte   // used when remote
}

func (r Ref) Tell(m GetState) error {
	if r.addr == "local" {
		r.mailbox <- m // the object itself, nothing copied
		return nil
	}
	b, err := json.Marshal(m) // remote: serialize first
	if err != nil {
		return err // the send fails on this node
	}
	fmt.Printf("  serialized GetState: %d bytes, send to %s\n", len(b), r.addr)
	r.wire <- b
	return nil
}

var replies = make(chan State, 4) // the API's own mailbox

func deviceActor(node string, mailbox chan GetState) {
	go func() {
		for range mailbox {
			replies <- State{"device 42", node}
		}
	}()
}

// node3 decodes bytes from the wire into device 42's real mailbox. drop simulates a blip.
func node3(wire chan []byte, mailbox chan GetState, drop *atomic.Bool) {
	go func() {
		for b := range wire {
			if drop.Load() {
				fmt.Println("  network: bytes lost between the nodes")
				continue
			}
			var m GetState
			_ = json.Unmarshal(b, &m)
			mailbox <- m
		}
	}()
}

func ask(r Ref, timeout time.Duration) (State, error) {
	if err := r.Tell(GetState{ReplyTo: "api@node1"}); err != nil {
		return State{}, err
	}
	select {
	case s := <-replies:
		return s, nil
	case <-time.After(timeout):
		return State{}, errors.New("ask timed out")
	}
}

func main() {
	fmt.Println("== Monday: device 42 is local")
	local := make(chan GetState, 8)
	deviceActor("node 1", local)
	ref := Ref{addr: "local", mailbox: local}
	s, err := ask(ref, time.Second)
	fmt.Printf("api: %v err=%v\n", s, err)
	err = ref.Tell(GetState{ReplyTo: "api@node1", Conn: make(chan int)})
	<-replies
	fmt.Printf("api: GetState with a connection inside, local send err=%v (works by luck)\n", err)

	fmt.Println("\n== Tuesday: sharding moved device 42 to node 3")
	remote := make(chan GetState, 8)
	wire := make(chan []byte, 8)
	var drop atomic.Bool
	deviceActor("node 3", remote)
	node3(wire, remote, &drop)
	ref = Ref{addr: "node3:4053", wire: wire} // only the address changed
	s, err = ask(ref, time.Second)
	fmt.Printf("api: %v err=%v (same line of code)\n", s, err)

	fmt.Println("\n== the Monday bug, now remote")
	err = ref.Tell(GetState{ReplyTo: "api@node1", Conn: make(chan int)})
	fmt.Printf("api: send failed on node 1: %v\n", err)

	fmt.Println("\n== a clean message, lost by the network")
	drop.Store(true)
	s, err = ask(ref, 500*time.Millisecond)
	fmt.Printf("api: %v err=%v (lost and slow look the same)\n", s, err)

	fmt.Println("\n== the fix: test-serialize every message, plan for loss")
	_, testErr := json.Marshal(GetState{ReplyTo: "api@node1"})
	fmt.Printf("test: GetState round-trips, err=%v\n", testErr)
	drop.Store(false)
	for try := 1; try <= 2; try++ {
		s, err = ask(ref, 500*time.Millisecond)
		fmt.Printf("api: try %d -> %v err=%v\n", try, s, err)
		if err == nil {
			break
		}
	}
}
