// Hybrid logical clocks and TrueTime. It replays the lesson: node B's clock
// runs 2 ms behind, stamps still respect cause, a jumping clock is refused,
// and Spanner's commit wait orders T1 and T2. Times are milliseconds after
// 10:00:00.
package main

import "fmt"

// Stamp is a hybrid logical clock value: wall time plus a counter.
type Stamp struct {
	T, C int
}

func (s Stamp) String() string { return fmt.Sprintf("(.%03d, %d)", s.T, s.C) }

// HLC is the clock one node keeps.
type HLC struct {
	last      Stamp
	maxOffset int // 0 means the guard is off
}

// Local runs on a local event or a send, with this node's wall clock.
func (h *HLC) Local(wall int) Stamp {
	if wall > h.last.T {
		h.last = Stamp{wall, 0} // time moved: the counter resets
	} else {
		h.last.C++ // time stood still: count instead
	}
	return h.last
}

// Receive merges a stamp from a message. It refuses a stamp too far ahead.
func (h *HLC) Receive(wall int, msg Stamp) (Stamp, bool) {
	if h.maxOffset > 0 && msg.T > wall+h.maxOffset {
		return h.last, false // the sender's clock is broken: do not follow it
	}
	t := max(wall, h.last.T, msg.T)
	c := 0 // the wall clock is ahead of both
	switch {
	case t == h.last.T && t == msg.T:
		c = max(h.last.C, msg.C) + 1
	case t == h.last.T:
		c = h.last.C + 1
	case t == msg.T:
		c = msg.C + 1
	}
	h.last = Stamp{t, c}
	return h.last, true
}

// Server is a Spanner server: its clock may be off by at most eps.
type Server struct {
	offset, eps int // clock = real + offset, and |offset| <= eps
}

// Now is TrueTime: the real time is surely inside [earliest, latest].
func (s Server) Now(real int) (earliest, latest int) {
	clock := real + s.offset
	return clock - s.eps, clock + s.eps
}

// Commit stamps a transaction. It returns the stamp and the reply time.
func (s Server) Commit(real int, commitWait bool) (stamp, reply int) {
	_, stamp = s.Now(real) // the latest possible time: never in the past
	reply = real
	if commitWait {
		for {
			earliest, _ := s.Now(reply)
			if earliest > stamp {
				break // the stamp is now surely in the past, everywhere
			}
			reply++ // sleep 1 ms
		}
	}
	return stamp, reply
}

func main() {
	fmt.Println("== Hybrid logical clock")
	a, b := &HLC{}, &HLC{}
	e1 := a.Local(5)
	fmt.Println("A event, wall .005:", e1)
	e2, _ := b.Receive(3, e1)
	fmt.Println("B receive, wall .003:", e2)
	fmt.Println("B event, wall .004:", b.Local(4))
	fmt.Println("B event, wall .006:", b.Local(6))

	fmt.Println("== One clock jumps 500 ms ahead")
	bad := b.Local(506)
	fmt.Println("B stamps, wall .506:", bad)
	a.Local(8)
	got, _ := a.Receive(8, bad)
	fmt.Println("no guard: A follows B:", got)
	guarded := &HLC{maxOffset: 250}
	guarded.Local(8)
	got, ok := guarded.Receive(8, bad)
	fmt.Printf("guard 250 ms: accepted %v, A stays %v\n", ok, got)

	fmt.Println("== TrueTime, error 4 ms")
	eu, us := Server{0, 4}, Server{-4, 4} // the US clock is 4 ms behind: inside its error
	e, l := eu.Now(10)
	fmt.Printf("Europe at real .010: interval .%03d to .%03d\n", e, l)
	t1, reply := eu.Commit(10, true)
	e, l = eu.Now(reply)
	fmt.Printf("T1 stamp .%03d, reply at real .%03d, interval then .%03d to .%03d\n", t1, reply, e, l)
	t2, _ := us.Commit(23, true)
	e, l = us.Now(23)
	fmt.Printf("T2 on the US server at real .023: interval .%03d to .%03d, stamp .%03d > .%03d\n", e, l, t2, t1)

	fmt.Println("== Commit wait switched off")
	t1, reply = eu.Commit(10, false)
	fmt.Printf("T1 stamp .%03d, reply at real .%03d\n", t1, reply)
	e, l = us.Now(12)
	t2, _ = us.Commit(12, false)
	fmt.Printf("T2 at real .012: interval .%03d to .%03d, stamp .%03d < .%03d: WRONG ORDER\n", e, l, t2, t1)

	fmt.Println("== Commit wait back on")
	t1, reply = eu.Commit(10, true)
	e, l = us.Now(20)
	t2, _ = us.Commit(20, true)
	fmt.Printf("T1 replies at real .%03d, T2 at real .020: interval .%03d to .%03d, stamp .%03d > .%03d\n", reply, e, l, t2, t1)

	fmt.Println("== Error grows to 10 ms")
	lost := Server{0, 10}
	e, l = lost.Now(15)
	s, reply := lost.Commit(15, true)
	fmt.Printf("at real .015: interval .%03d to .%03d, stamp .%03d, reply .%03d, waited %d ms\n", e, l, s, reply, reply-15)
}
