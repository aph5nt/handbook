// Supervision: a child goroutine does not catch its own panic. The runtime recovers it
// and asks the parent's decider: Resume, Restart, Stop or Escalate.
// The last part replays mailbox overflow as arithmetic over simulated seconds.
package main

import (
	"fmt"
	"strings"
)

type Directive int

const (
	Resume Directive = iota
	Restart
	Stop
	Escalate
)

func (d Directive) String() string { return [...]string{"Resume", "Restart", "Stop", "Escalate"}[d] }

type Child struct {
	name     string
	readings int
	mailbox  chan string
}

// handle panics on a corrupt packet: let it crash, no defensive code here.
func (c *Child) handle(m string) {
	if m == "corrupt" {
		panic("parse error")
	}
	c.readings++
}

// runOne is the runtime for one message: recover turns a panic into "I failed".
func runOne(c *Child, m string) (failed bool) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("  %s: failed on %q: %v, tells its supervisor\n", c.name, m, r)
			failed = true
		}
	}()
	c.handle(m)
	return false
}

// supervise is the actor's goroutine: it ranges over the mailbox channel and applies
// the decider to every failure.
func supervise(c *Child, decide func() Directive, siblings []*Child) {
	for m := range c.mailbox { // the failing message is already off the queue
		if !runOne(c, m) {
			continue
		}
		switch d := decide(); d {
		case Resume:
			fmt.Printf("  supervisor: %v, state kept: readings %d\n", d, c.readings)
		case Restart:
			c.readings = 0 // a fresh instance, the same mailbox
			fmt.Printf("  supervisor: %v, fresh instance, mailbox kept: %d waiting\n", d, len(c.mailbox))
		case Stop:
			fmt.Printf("  supervisor: %v, %d queued messages to dead letters\n", d, len(c.mailbox))
			return
		case Escalate:
			fmt.Printf("  supervisor: %v -> /user restarts the supervisor and all its children\n", d)
			for _, s := range siblings {
				s.readings = 0
			}
			return
		}
	}
}

func mailboxOf(msgs ...string) chan string {
	mb := make(chan string, len(msgs))
	for _, m := range msgs {
		mb <- m
	}
	close(mb)
	return mb
}

func newTree() (d41, d42, d43 *Child) {
	d41 = &Child{name: "device 41", readings: 7}
	d42 = &Child{name: "device 42", readings: 4, mailbox: mailboxOf("21.5 C", "corrupt", "21.7 C", "21.9 C")}
	d43 = &Child{name: "device 43", readings: 9}
	return
}

func main() {
	for _, d := range []Directive{Resume, Restart, Stop, Escalate} {
		fmt.Printf("== %v\n", d)
		d41, d42, d43 := newTree()
		dir := d
		done := make(chan struct{})
		go func() { // the actor's goroutine
			supervise(d42, func() Directive { return dir }, []*Child{d41, d42, d43})
			close(done)
		}()
		<-done
		fmt.Printf("  readings: 41=%d 42=%d 43=%d\n", d41.readings, d42.readings, d43.readings)
	}

	fmt.Println("== poisoned message: retried on every start, limit 3 restarts in 1 minute")
	backoff := 1
	for n := 1; ; n++ {
		if n > 3 {
			fmt.Println("  supervisor: limit reached, Stop device 42, raise an alert")
			break
		}
		c := &Child{name: "device 42"}
		runOne(c, "corrupt")
		fmt.Printf("  supervisor: restart %d of 3 after %d s backoff\n", n, backoff)
		backoff *= 2
	}

	fmt.Println("== mailbox overflow: 1,000 readings/s in, 200/s handled, a plain tell")
	in, out := 1000, 200
	for _, t := range []int{1, 60, 3600} {
		fmt.Printf("  after %4d s: queued %d\n", t, (in-out)*t)
	}
	fmt.Println("  bounded: capacity 1,000, drop oldest")
	queue, dropped := 0, 0
	for t := 1; t <= 3600; t++ {
		queue += in
		if queue > 1000 {
			dropped += queue - 1000
			queue = 1000
		}
		queue -= min(out, queue)
	}
	fmt.Printf("  after 3600 s: queued at most %d, %s dropped as oldest\n", 1000, group(dropped))
}

func group(n int) string {
	s := fmt.Sprint(n)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}
