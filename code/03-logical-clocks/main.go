// Logical clocks. Replays the lesson: a log sorted by wall-clock stamps puts
// a ban before the post it reacts to; a Lamport clock orders events by
// cause; a vector clock also tells concurrent events apart.
package main

import (
	"fmt"
	"sort"
	"time"
)

// LogLine is one event in the merged log.
type LogLine struct {
	stamp time.Time // the server's wall clock, which may drift
	text  string
}

// SortByWall orders the merged log by wall-clock stamp, as most log tools do.
func SortByWall(lines []LogLine) {
	sort.Slice(lines, func(i, j int) bool { return lines[i].stamp.Before(lines[j].stamp) })
}

// Lamport is one node's counter.
type Lamport struct{ c int }

// Event is a local event or a send: add one.
func (l *Lamport) Event() int { l.c++; return l.c }

// Receive takes the larger of the own and the message counter, then adds one.
func (l *Lamport) Receive(msg int) int {
	if msg > l.c {
		l.c = msg
	}
	l.c++
	return l.c
}

// Vector is one counter per node: [A, B, C].
type Vector [3]int

// Tick adds one to the node's own slot.
func (v *Vector) Tick(me int) Vector { v[me]++; return *v }

// Merge takes the larger value in every slot, then ticks the own slot.
func (v *Vector) Merge(me int, msg Vector) Vector {
	for k := range v {
		if msg[k] > v[k] {
			v[k] = msg[k]
		}
	}
	return v.Tick(me)
}

// Compare says "before", "after" or "concurrent".
func Compare(a, b Vector) string {
	le, ge := true, true
	for k := range a {
		if a[k] > b[k] {
			le = false
		}
		if a[k] < b[k] {
			ge = false
		}
	}
	switch {
	case le && !ge:
		return "before"
	case ge && !le:
		return "after"
	}
	return "concurrent"
}

func main() {
	fmt.Println("== Wall clocks")
	t0 := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	skew1 := 10 * time.Millisecond                                     // server 1 runs 10 ms fast
	post := LogLine{t0.Add(0*time.Millisecond + skew1), "user posted"} // real 0 ms
	ban := LogLine{t0.Add(3 * time.Millisecond), "user banned"}        // real 3 ms
	log := []LogLine{post, ban}
	SortByWall(log)
	for _, l := range log {
		fmt.Printf("%s  %s\n", l.stamp.Format("15:04:05.000"), l.text)
	}
	fmt.Println("the log shows the ban first: a banned user posted")
	var s1, s2 Lamport
	p := s1.Event()
	b := s2.Receive(p) // the moderator saw the post
	fmt.Printf("fix by cause: post %d, ban %d\n", p, b)

	fmt.Println("== Lamport")
	var A, B, C Lamport
	a1 := A.Event()
	b2 := B.Receive(a1)
	c1 := C.Event()
	c3 := C.Receive(b2)
	fmt.Printf("A event %d, B receive max(0, %d)+1 = %d, C own %d, C receive max(1, %d)+1 = %d\n", a1, a1, b2, c1, b2, c3)
	fmt.Printf("A's %d and C's %d: same number, no cause between them\n", a1, c1)

	fmt.Println("== Vector")
	var va, vb, vc Vector
	e1 := va.Tick(0)
	e2 := vb.Merge(1, e1)
	e3 := vc.Tick(2)
	e4 := vc.Merge(2, e2)
	fmt.Printf("A %v, B %v, C own %v, C receive %v\n", e1, e2, e3, e4)
	fmt.Printf("A %v vs C %v: %s\n", e1, e3, Compare(e1, e3))
	fmt.Printf("A %v vs C %v: %s\n", e1, e4, Compare(e1, e4))
}
