// Erlang OTP in Go: a gen_server (the library runs the loop, you write handle_call,
// handle_cast and handle_info) and a one_for_one supervisor with a restart intensity.
// Time is scaled: 100 ms stands for the 5 s default call timeout.
package main

import (
	"errors"
	"fmt"
	"time"
)

const callTimeout = 100 * time.Millisecond // stands for 5 s

var errCallTimeout = errors.New("exit: call timeout")

type envelope struct {
	kind  string // "call", "cast" or "info"
	msg   string
	reply chan string
}

// Callbacks is what you write. The library below is what OTP gives you.
type Callbacks interface {
	HandleCall(msg string) string
	HandleCast(msg string)
	HandleInfo(msg string)
}

type GenServer struct {
	mailbox chan envelope
}

// start runs the loop. A crash in a callback becomes an exit signal to the linked parent.
func start(cb Callbacks, exit chan<- error) *GenServer {
	g := &GenServer{mailbox: make(chan envelope, 64)}
	go func() {
		defer func() {
			if r := recover(); r != nil && exit != nil {
				exit <- fmt.Errorf("%v", r) // the link carries the death
			}
		}()
		for e := range g.mailbox { // one message at a time
			switch e.kind {
			case "call":
				e.reply <- cb.HandleCall(e.msg)
			case "cast":
				cb.HandleCast(e.msg)
			case "info":
				cb.HandleInfo(e.msg)
			}
		}
	}()
	return g
}

// Call is ask: the caller blocks, with a timeout.
func (g *GenServer) Call(msg string) (string, error) {
	reply := make(chan string, 1)
	g.mailbox <- envelope{"call", msg, reply}
	select {
	case r := <-reply:
		return r, nil
	case <-time.After(callTimeout):
		return "", errCallTimeout
	}
}

// Cast is tell: into the mailbox, the sender continues at once.
func (g *GenServer) Cast(msg string) { g.mailbox <- envelope{kind: "cast", msg: msg} }

// Info is any plain message: a timer, a monitor.
func (g *GenServer) Info(msg string) { g.mailbox <- envelope{kind: "info", msg: msg} }

// Queue is the orders queue's callbacks.
type Queue struct {
	name   string
	stored int
	acks   int
}

func (q *Queue) HandleCall(msg string) string {
	if msg == "long job" {
		time.Sleep(3 * callTimeout / 2) // stands for 7.5 s
		return "ok"
	}
	q.stored++
	return "ok"
}

func (q *Queue) HandleCast(msg string) {
	if msg == "bad" {
		panic("cannot handle: bad message") // no defensive code: let it crash
	}
	q.acks++
	fmt.Printf("  %s: handle_cast %s, acks %d\n", q.name, msg, q.acks)
}

func (q *Queue) HandleInfo(msg string) {
	if msg == "other work" {
		return
	}
	fmt.Printf("  %s: handle_info %s, expired 0 messages\n", q.name, msg)
}

// oneForOne restarts only the crashed child. It counts restarts in the period and gives
// up past the intensity: it exits too, so its own parent decides.
func oneForOne(name string, intensity int, period time.Duration, exit chan error, restart func()) error {
	var recent []time.Time
	for reason := range exit {
		now := time.Now()
		kept := recent[:0]
		for _, t := range recent {
			if now.Sub(t) < period {
				kept = append(kept, t)
			}
		}
		recent = append(kept, now)
		if len(recent) > intensity {
			return fmt.Errorf("%s: restart %d in 10 s is past the intensity of %d: %v", name, len(recent), intensity, reason)
		}
		restart() // a fresh child, no sibling touched
	}
	return nil
}

// partOf splits the orders queue by key: A to M, then N to Z.
func partOf(key string) int {
	if key[0] <= 'M' {
		return 1
	}
	return 2
}

func main() {
	fmt.Println("== gen_server: call, cast, info")
	q := &Queue{name: "orders"}
	orders := start(q, nil)
	r, err := orders.Call("publish")
	fmt.Printf("  client: call publish -> %s, err=%v, stored %d\n", r, err, q.stored)
	orders.Cast("ack")
	orders.Info("timer: expire")
	time.Sleep(10 * time.Millisecond)

	fmt.Println("== the call timeout: orders is busy with a long job")
	go orders.Call("long job")
	time.Sleep(5 * time.Millisecond)
	for i := 0; i < 39; i++ {
		orders.Info("other work")
	}
	fmt.Printf("  orders: busy, mailbox %d\n", len(orders.mailbox)+1)
	r, err = orders.Call("publish")
	fmt.Printf("  client: reply %q, %v: the caller crashes, its own supervisor decides\n", r, err)
	time.Sleep(100 * time.Millisecond)

	fmt.Println("== one_for_one: only orders restarts")
	exit := make(chan error, 1)
	badOrders := start(&Queue{name: "orders"}, exit)
	emails := start(&Queue{name: "emails"}, nil)
	badOrders.Cast("bad")
	reason := <-exit
	fmt.Printf("  queue supervisor: exit signal from orders: %v\n", reason)
	badOrders = start(&Queue{name: "orders"}, exit)
	fmt.Println("  queue supervisor: fresh orders process")
	reply, _ := emails.Call("publish")
	fmt.Printf("  emails: never noticed, call -> %s\n", reply)

	fmt.Println("== a crash loop: intensity 10 restarts in 10 s")
	badOrders.Cast("bad") // the bad message keeps coming
	gaveUp := oneForOne("queue supervisor", 10, 10*time.Second, exit, func() {
		badOrders = start(&Queue{name: "orders"}, exit)
		badOrders.Cast("bad")
	})
	fmt.Printf("  %v\n", gaveUp)
	fmt.Println("  top supervisor: exit signal, restart the queue subtree: fresh orders, fresh emails")

	fmt.Println("== one process everyone calls: 1,000 publishers, 1,500 messages/s (assumed)")
	in, capacity := 1500, 1000 // one process, one core, 1 ms per message
	for _, t := range []int{10, 50} {
		backlog := (in - capacity) * t
		fmt.Printf("  after %2d s: mailbox %d, a new call waits %d s (timeout 5 s)\n", t, backlog, backlog/capacity)
	}
	fmt.Println("== split by key into two processes: 750 messages/s each")
	for _, key := range []string{"Anna", "Nora"} {
		fmt.Printf("  key %s -> part %d\n", key, partOf(key))
	}
	fmt.Printf("  each part: %d in, %d capacity, backlog 0, 2 of 8 cores busy\n", in/2, capacity)
}
