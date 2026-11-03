// Priority queue: an urgent email jumps 10,000 newsletters, then starvation.
//
// container/heap stands in for a RabbitMQ queue declared with
// x-max-priority. Higher priority first; inside one priority, first in,
// first out (by sequence number). The worker sends 3 emails a second.
package main

import (
	"container/heap"
	"fmt"
)

type Email struct {
	Kind     string
	Priority int
	seq      int
}

type PQ []Email

func (q PQ) Len() int { return len(q) }
func (q PQ) Less(i, j int) bool {
	if q[i].Priority != q[j].Priority {
		return q[i].Priority > q[j].Priority // higher priority first
	}
	return q[i].seq < q[j].seq // same priority: first in, first out
}
func (q PQ) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *PQ) Push(x any)   { *q = append(*q, x.(Email)) }
func (q *PQ) Pop() any {
	old := *q
	e := old[len(old)-1]
	*q = old[:len(old)-1]
	return e
}

const perSecond = 3 // the worker sends 3 emails a second

// position returns how many emails go out before (and including) the reset.
func position(prio bool, prefetch int) int {
	q := &PQ{}
	seq := 0
	add := func(kind string, p int) {
		if !prio {
			p = 0 // a plain queue ignores priority
		}
		seq++
		heap.Push(q, Email{kind, p, seq})
	}
	for i := 0; i < 10000; i++ {
		add("news", 1)
	}
	// The worker already holds `prefetch` newsletters in its buffer.
	sent := 0
	for i := 0; i < prefetch; i++ {
		heap.Pop(q)
		sent++
	}
	add("reset", 9)
	for q.Len() > 0 {
		e := heap.Pop(q).(Email)
		sent++
		if e.Kind == "reset" {
			return sent
		}
	}
	return -1
}

func main() {
	fmt.Println("== Tab 1: jump the line ==")
	p := position(false, 0)
	fmt.Printf("step 3: plain queue: reset is email %d, sent after %.0f minutes\n", p, float64(p)/perSecond/60)
	p = position(true, 0)
	fmt.Printf("step 4: priority 9 vs 1: reset is email %d, sent within %d second\n", p, (p+perSecond-1)/perSecond)
	p = position(true, 50)
	fmt.Printf("step 5: with prefetch 50 the reset is email %d: prefetched emails are not reordered\n", p)

	fmt.Println("== Tab 2: starvation and separate queues ==")
	q := &PQ{}
	seq := 0
	for i := 0; i < 10000; i++ {
		seq++
		heap.Push(q, Email{"news", 1, seq})
	}
	newsSent := 0
	for sec := 0; sec < 3600; sec++ { // one hour of the marketing push
		for k := 0; k < perSecond; k++ { // 3 urgent emails arrive every second
			seq++
			heap.Push(q, Email{"code", 9, seq})
		}
		for k := 0; k < perSecond; k++ {
			if heap.Pop(q).(Email).Kind == "news" {
				newsSent++
			}
		}
	}
	fmt.Printf("step 2: one hour, 3 urgent a second, worker sends 3 a second: newsletters sent %d\n", newsSent)

	// Separate queues, each with its own worker.
	urgent, bulk := 0, 10000
	urgentSent, bulkSent := 0, 0
	for sec := 0; sec < 3600; sec++ {
		urgent += perSecond
		n := min(urgent, perSecond)
		urgent -= n
		urgentSent += n
		m := min(bulk, perSecond)
		bulk -= m
		bulkSent += m
	}
	depth := map[string]int{"urgent": urgent, "bulk": bulk} // alert on each queue
	fmt.Printf("step 4: two queues, one hour: urgent sent %d (waiting %d), newsletters sent %d (waiting %d)\n",
		urgentSent, depth["urgent"], bulkSent, depth["bulk"])
}
