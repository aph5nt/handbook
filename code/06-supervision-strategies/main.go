// Supervision strategies: the directive says what happens, the strategy says to whom.
// Each child is a goroutine that ranges over its mailbox channel.
package main

import "fmt"

type Strategy int

const (
	OneForOne Strategy = iota
	AllForOne
	RestForOne
)

func (s Strategy) String() string {
	return [...]string{"one-for-one", "all-for-one", "rest-for-one"}[s]
}

type Msg interface{}

type Boom struct{ Reason string }
type Restart struct {
	Socket int // 0 keeps the socket the child already holds
	Ack    chan struct{}
}
type Get struct{ Reply chan string }
type Failed struct {
	Child  *Child
	Reason string
	Done   chan struct{}
}

type Child struct {
	name     string
	mailbox  chan Msg
	socket   int
	restarts int
	parent   chan Msg
}

func (c *Child) run() {
	for m := range c.mailbox { // one message at a time
		switch v := m.(type) {
		case Boom:
			c.parent <- Failed{Child: c, Reason: v.Reason}
		case Restart:
			c.restarts++ // a fresh instance on the same mailbox
			if v.Socket != 0 {
				c.socket = v.Socket
			}
			v.Ack <- struct{}{}
		case Get:
			v.Reply <- fmt.Sprintf("%s: restarts %d, socket %d", c.name, c.restarts, c.socket)
		}
	}
}

type Supervisor struct {
	name     string
	strategy Strategy
	children []*Child
	mailbox  chan Msg
	count    int // restarts in the window
	limit    int
	socket   int // the socket a group shares, when it has one
	newConn  bool
	done     chan string
}

// targets is the strategy: which children one failure hits.
func (s *Supervisor) targets(failed *Child) []*Child {
	switch s.strategy {
	case AllForOne:
		return s.children
	case RestForOne:
		for i, c := range s.children {
			if c == failed {
				return s.children[i:] // the child and all started after it
			}
		}
	}
	return []*Child{failed}
}

func (s *Supervisor) run() {
	for m := range s.mailbox {
		f := m.(Failed)
		if s.count >= s.limit { // the limit: 3 restarts in 1 minute
			s.done <- fmt.Sprintf("%s: failure past the limit %d, stop %s, escalate", s.name, s.limit, f.Child.name)
			continue
		}
		s.count++
		sock := 0
		if s.newConn { // on start the group opens a new socket
			s.socket++
			sock = s.socket
		}
		hit := s.targets(f.Child)
		for _, c := range hit {
			ack := make(chan struct{})
			c.mailbox <- Restart{sock, ack}
			<-ack
		}
		s.done <- fmt.Sprintf("%s: %v, %d restarted after %s failed, restarts %d of %d", s.name, s.strategy, len(hit), f.Child.name, s.count, s.limit)
	}
}

func newSup(name string, st Strategy, newConn bool, names ...string) *Supervisor {
	s := &Supervisor{name: name, strategy: st, mailbox: make(chan Msg, 64), limit: 3, socket: 1, newConn: newConn, done: make(chan string, 64)}
	for _, n := range names {
		c := &Child{name: n, mailbox: make(chan Msg, 8), parent: s.mailbox, socket: 1}
		s.children = append(s.children, c)
		go c.run()
	}
	go s.run()
	return s
}

func (s *Supervisor) fail(i int) {
	s.children[i].mailbox <- Boom{"corrupt"}
	fmt.Println(<-s.done)
}

func (s *Supervisor) show() {
	for _, c := range s.children {
		r := make(chan string)
		c.mailbox <- Get{r}
		fmt.Println(" ", <-r)
	}
}

func main() {
	fmt.Println("== one-for-one on independent devices")
	d := newSup("devices", OneForOne, false, "device 41", "device 42", "device 43")
	d.fail(1)
	d.show()
	d.fail(1)
	d.fail(1)
	d.fail(1) // the fourth failure

	fmt.Println("\n== all-for-one on a group that shares a socket")
	g := newSup("connection", AllForOne, true, "reader", "writer", "heartbeat")
	g.fail(0)
	g.show()

	fmt.Println("\n== rest-for-one: the writer fails, the reader keeps going")
	r := newSup("connection", RestForOne, false, "reader", "writer", "heartbeat")
	r.fail(1)
	r.show()

	fmt.Println("\n== wrong: all-for-one on 10,000 independent devices")
	names := make([]string, 10000)
	for i := range names {
		names[i] = fmt.Sprintf("device %d", i)
	}
	fleet := newSup("devices", AllForOne, false, names...)
	fleet.fail(42)

	fmt.Println("\n== wrong: one-for-one on the shared socket")
	w := newSup("connection", OneForOne, true, "reader", "writer", "heartbeat")
	w.fail(0)
	w.show()
	fmt.Println("  the writer writes to socket 1, which is dead: every write fails")
}
