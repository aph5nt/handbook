// Lifecycle hooks and death watch. The runtime wraps the handler: PreStart before the
// first message, PostStop after the last, a restart builds a fresh instance on the same
// mailbox, and a stop sends Terminated to every watcher.
package main

import (
	"errors"
	"fmt"
)

type Msg interface{}

type Fail struct{}
type Stop struct{}
type State struct{ C float64 }
type Terminated struct{ Who string }

var openConns int // thermostat connections open in the whole process

// Device is one instance. A restart throws it away and builds a new one.
type Device struct{ leaky bool }

func newDevice(leaky bool) *Device {
	d := &Device{leaky: leaky}
	if leaky {
		openConns++ // opened in the constructor, closed nowhere
	}
	return d
}
func (d *Device) PreStart() {
	if !d.leaky {
		openConns++
	}
}
func (d *Device) PostStop() {
	if !d.leaky {
		openConns--
	}
}

// run is the runtime: one mailbox, many instances over time.
func run(name string, mailbox chan Msg, leaky bool, watchers *[]chan Msg, done chan struct{}) {
	d := newDevice(leaky)
	d.PreStart()
	fmt.Printf("%s: created, PreStart, open connections %d\n", name, openConns)
	for m := range mailbox {
		switch m.(type) {
		case Fail:
			d.PostStop() // PreRestart on the old instance calls PostStop
			d = newDevice(leaky)
			d.PreStart() // PostRestart calls PreStart
			fmt.Printf("%s: failed, restarted on the same mailbox, open connections %d\n", name, openConns)
		case State:
			for _, w := range *watchers {
				w <- m
			}
		case Stop:
			d.PostStop()
			fmt.Printf("%s: stopped, PostStop, open connections %d\n", name, openConns)
			for _, w := range *watchers {
				w <- Terminated{name} // death watch: exactly once per watcher
			}
			close(done)
			return
		}
	}
}

// session is a watcher. handles says whether its behaviour handles Terminated.
func session(user string, inbox chan Msg, handles bool, out chan string) {
	go func() {
		for m := range inbox {
			switch v := m.(type) {
			case State:
				fmt.Printf("session %s: shows %.1f C\n", user, v.C)
			case Terminated:
				if !handles {
					out <- fmt.Sprintf("session %s: failed: %v", user, errors.New("DeathPactException: unhandled Terminated("+v.Who+")"))
					return
				}
				out <- fmt.Sprintf("session %s: Terminated(%s) -> shows offline, unwatch", user, v.Who)
				return
			}
		}
	}()
}

func main() {
	fmt.Println("== lifecycle")
	mb := make(chan Msg, 8)
	done := make(chan struct{})
	none := []chan Msg{}
	go run("device 42", mb, false, &none, done)
	mb <- Fail{}
	mb <- Stop{}
	<-done

	fmt.Println("\n== the leak: open in the constructor, close nowhere")
	openConns = 0
	mb = make(chan Msg, 8)
	done = make(chan struct{})
	go run("device 42", mb, true, &none, done)
	mb <- Fail{}
	mb <- Fail{}
	mb <- Stop{}
	<-done
	fmt.Printf("after 3 instances: %d open, %d belong to dead instances\n", openConns, openConns-1)

	for _, handles := range []bool{true, false} {
		if handles {
			fmt.Println("\n== death watch, Terminated handled")
		} else {
			fmt.Println("\n== death watch, Terminated not handled")
		}
		openConns = 0
		out := make(chan string, 2)
		ana, ben := make(chan Msg, 4), make(chan Msg, 4)
		session("Ana", ana, handles, out)
		session("Ben", ben, handles, out)
		watchers := []chan Msg{ana, ben} // Watch(42) from both sessions
		mb = make(chan Msg, 8)
		done = make(chan struct{})
		go run("device 42", mb, false, &watchers, done)
		mb <- State{21.5}
		mb <- Stop{}
		<-done
		fmt.Println(<-out)
		fmt.Println(<-out)
	}
}
