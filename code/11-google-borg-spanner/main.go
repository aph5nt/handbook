// Command google replays the Google lesson: a Borg-style scheduler, the
// TrueTime commit wait, and an SRE error budget. Each is a small model.
package main

import "fmt"

const Slots = 4

type Machine struct {
	Prod  []string
	Batch []string
	Ports map[int]string
	Dead  bool
}

type Borg struct {
	M     []*Machine
	Queue []string
}

func (b *Borg) Place(copyName string) int {
	best := -1
	for i, m := range b.M {
		if !m.Dead && len(m.Prod) < Slots && (best < 0 || len(m.Prod) < len(b.M[best].Prod)) {
			best = i
		}
	}
	m := b.M[best]
	if len(m.Prod)+len(m.Batch) == Slots {
		last := len(m.Batch) - 1
		b.Queue = append(b.Queue, m.Batch[last])
		m.Batch = m.Batch[:last]
	}
	m.Prod = append(m.Prod, copyName)
	return best
}

func (b *Borg) Submit(job string, copies int) {
	for c := 1; c <= copies; c++ {
		b.Place(fmt.Sprintf("%s %d", job, c))
	}
}

func (b *Borg) Fill() {
	for _, m := range b.M {
		for !m.Dead && len(b.Queue) > 0 && len(m.Prod)+len(m.Batch) < Slots {
			m.Batch = append(m.Batch, b.Queue[0])
			b.Queue = b.Queue[1:]
		}
	}
}

func (b *Borg) Die(i int) {
	m := b.M[i]
	m.Dead = true
	b.Queue = append(b.Queue, m.Batch...)
	for _, c := range m.Prod {
		b.Place(c)
	}
}

func Port(m *Machine, job string, want, other int) int {
	if _, taken := m.Ports[want]; taken {
		want = other
	}
	m.Ports[want] = job
	return want
}

func TrueTimeCommit(trueNow, doubt int) (stamp, doneAt int) {
	stamp = trueNow + doubt
	for trueNow <= stamp {
		trueNow++
	}
	return stamp, trueNow
}

func PlainStamp(trueNow, behind int) int { return trueNow - behind }

func Budget(left int, failures ...int) (int, bool) {
	for _, f := range failures {
		left -= f
	}
	return left, left <= 0
}

func main() {
	b := &Borg{}
	for i := 0; i < 3; i++ {
		b.M = append(b.M, &Machine{Ports: map[int]string{}})
	}
	show := func(step string) {
		fmt.Println(step)
		for i, m := range b.M {
			if !m.Dead {
				fmt.Printf("  machine %d: production %v, batch %v\n", i+1, m.Prod, m.Batch)
			}
		}
		fmt.Printf("  queue: %d batch jobs\n", len(b.Queue))
	}
	b.Submit("Search", 3)
	b.Submit("Gmail", 2)
	show("borg step 2: packed")
	for i := 1; i <= 7; i++ {
		b.Queue = append(b.Queue, fmt.Sprintf("b%d", i))
	}
	b.Fill()
	show("borg step 3: batch fills the gaps")
	b.Die(1)
	show("borg step 4-5: machine 2 dies, its copies move")
	fmt.Printf("borg step 6: machine 1, Search takes %d, Gmail gets %d\n", Port(b.M[0], "Search", 8080, 31207), Port(b.M[0], "Gmail", 8080, 31207))

	stamp, done := TrueTimeCommit(0, 6)
	fmt.Printf("spanner: stamp .%03d, released at .%03d\n", stamp, done)
	fmt.Printf("spanner, plain clock: Ana stamped .010 and released. A write in Iowa at true .012 gets stamp .%03d, earlier\n", PlainStamp(12, 5))

	left, frozen := Budget(43, 4)
	fmt.Println("sre: release 1:", left, frozen)
	left, frozen = Budget(left, 6, 7)
	fmt.Println("sre: releases 2 and 3:", left, frozen)
	left, frozen = Budget(left, 26)
	fmt.Println("sre: 26 min outage:", left, frozen)
}
