// Mailbox types: first in first out, priority, bounded. The actor code never changes.
package main

import (
	"container/heap"
	"fmt"
	"sync"
	"time"
)

type Msg struct {
	Kind string // "alarm" or "reading"
	seq  int    // arrival number: keeps order inside one priority
}

// Mailbox is what an actor takes from. The type decides the order.
type Mailbox interface {
	Put(Msg)
	Take() Msg
	Depth() int
}

// Fifo is the default: a channel.
type Fifo struct{ ch chan Msg }

func (f Fifo) Put(m Msg)  { f.ch <- m }
func (f Fifo) Take() Msg  { return <-f.ch }
func (f Fifo) Depth() int { return len(f.ch) }

// Prio sorts by priority, then arrival.
type Prio struct {
	mu   sync.Mutex
	cond *sync.Cond
	q    queue
	next int
}

type queue []Msg

func rank(m Msg) int {
	if m.Kind == "alarm" {
		return 0 // alarms before readings
	}
	return 1
}
func (q queue) Len() int { return len(q) }
func (q queue) Less(i, j int) bool {
	if rank(q[i]) != rank(q[j]) {
		return rank(q[i]) < rank(q[j])
	}
	return q[i].seq < q[j].seq
}
func (q queue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *queue) Push(x any)   { *q = append(*q, x.(Msg)) }
func (q *queue) Pop() any {
	o := *q
	m := o[len(o)-1]
	*q = o[:len(o)-1]
	return m
}

func newPrio() *Prio { p := &Prio{}; p.cond = sync.NewCond(&p.mu); return p }
func (p *Prio) Put(m Msg) {
	p.mu.Lock()
	m.seq = p.next
	p.next++
	heap.Push(&p.q, m)
	p.mu.Unlock()
	p.cond.Signal()
}
func (p *Prio) Take() Msg {
	p.mu.Lock()
	defer p.mu.Unlock()
	for p.q.Len() == 0 {
		p.cond.Wait()
	}
	return heap.Pop(&p.q).(Msg)
}
func (p *Prio) Depth() int { p.mu.Lock(); defer p.mu.Unlock(); return p.q.Len() }

// flood replays the story: 10,000 readings, the actor starts the first, the alarm arrives.
// It returns how many readings were handled before the alarm.
func flood(mb Mailbox) int {
	for i := 0; i < 10000; i++ {
		mb.Put(Msg{Kind: "reading"})
	}
	mb.Take() // the actor is handling this one
	mb.Put(Msg{Kind: "alarm"})
	before := 1
	for mb.Take().Kind != "alarm" {
		before++
	}
	return before
}

func starve(collapse bool) (readings, alarms int) {
	p := newPrio()
	for i := 0; i < 10000; i++ {
		p.Put(Msg{Kind: "reading"})
	}
	sent := false
	fire := func() { // the faulty sensor fires once per message handled
		if collapse && sent {
			return // the sender collapses repeats of one incident into one alarm
		}
		sent = true
		p.Put(Msg{Kind: "alarm"})
	}
	fire()
	for handled := 0; handled < 1000; handled++ { // look at the next 1,000 messages
		if p.Take().Kind == "alarm" {
			alarms++
		} else {
			readings++
		}
		fire()
	}
	return readings, alarms
}

func bounded(capacity, flood int, pushTimeout time.Duration) (accepted, dead int, blocked time.Duration) {
	mailbox := make(chan Msg, capacity) // the actor is busy and takes nothing here
	for i := 0; i < flood; i++ {
		select {
		case mailbox <- Msg{Kind: "reading"}:
			accepted++
			continue
		default:
		}
		if pushTimeout > 0 {
			start := time.Now()
			select {
			case mailbox <- Msg{Kind: "reading"}:
				accepted++
				continue
			case <-time.After(pushTimeout):
				blocked += time.Since(start)
			}
		}
		dead++
	}
	return
}

func main() {
	fmt.Println("== first in, first out")
	fmt.Printf("  readings handled before the alarm: %d\n", flood(Fifo{make(chan Msg, 10001)}))

	fmt.Println("== priority: alarms before readings")
	fmt.Printf("  readings handled before the alarm: %d (the one in progress)\n", flood(newPrio()))

	fmt.Println("== starvation: a faulty sensor fires an alarm per message handled")
	r, a := starve(false)
	fmt.Printf("  next 1,000 messages: %d alarms, %d readings\n", a, r)
	r, a = starve(true)
	fmt.Printf("  sender collapses repeats: %d alarm, %d readings\n", a, r)

	fmt.Println("== bounded, capacity 1,000, flood of 10,000")
	acc, dead, _ := bounded(1000, 10000, 0)
	fmt.Printf("  accepted %d, dead letters %d (reading 1,001 is the first refused)\n", acc, dead)
	_, _, blocked := bounded(1000, 1001, 50*time.Millisecond)
	fmt.Printf("  push timeout 50 ms: one refused reading held the sender's thread for about %v\n", blocked.Round(10*time.Millisecond))
	fmt.Printf("  9,000 refused readings x 50 ms = %d s of a shared thread\n", 9000*50/1000)
}
