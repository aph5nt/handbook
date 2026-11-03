// Command snowflake replays the Snowflake lesson with a simulated clock:
// the bit layout, the sequence in one millisecond, two workers, sorting by
// time, a clock that moves back, 4,096 ids per ms, and a reused machine id.
package main

import "fmt"

// 10:00:00.123 as milliseconds since the Twitter epoch of November 2010.
const t123 int64 = 500000000123

type Clock struct{ Now int64 }

type Worker struct {
	Machine, Seq, Last int64
	Clock              *Clock
	Waited             int64
}

func NewWorker(assigned int64, c *Clock) *Worker {
	return &Worker{Machine: assigned, Last: -1, Clock: c}
}

func (w *Worker) Next() int64 {
	now := w.Clock.Now
	for now < w.Last {
		w.tick()
		now = w.Clock.Now
	}
	if now == w.Last {
		w.Seq++
		if w.Seq == 4096 {
			w.tick()
			now = w.Clock.Now
			w.Seq = 0
		}
	} else {
		w.Seq = 0
	}
	w.Last = now
	return now<<22 | w.Machine<<12 | w.Seq
}

func (w *Worker) tick() { w.Clock.Now++; w.Waited++ } // simulated wait of 1 ms

func parts(id int64) string {
	return fmt.Sprintf("...%03d | %d | %d", (id>>22)%1000, (id>>12)&1023, id&4095)
}

func main() {
	c := &Clock{Now: t123}
	w7 := NewWorker(7, c)
	fmt.Println("make tab")
	a := w7.Next()
	fmt.Printf("step 1-4: Ana on worker 7: %s = %d\n", parts(a), a)
	b := w7.Next()
	fmt.Printf("step 5: Ben, same ms: %s\n", parts(b))
	w12 := NewWorker(12, c)
	d := w12.Next()
	fmt.Printf("step 6: worker 12, same ms: %s\n", parts(d))
	c.Now++
	e := w7.Next()
	fmt.Printf("step 7: next ms: %s, bigger than all three: %v\n", parts(e), e > a && e > b && e > d)
	o := NewWorker(3, &Clock{Now: t123})
	for i := 0; i < 4096; i++ {
		o.Next()
	}
	last := o.Next()
	fmt.Printf("  4,096 ids in one ms; id 4,097 waits %d ms: %s\n", o.Waited, parts(last))

	fmt.Println("fail tab")
	c = &Clock{Now: t123}
	w := NewWorker(7, c)
	fmt.Printf("step 1: last id %s\n", parts(w.Next()))
	c.Now = t123 - 5
	fmt.Printf("step 2: clock moved back 5 ms, now ...%03d\n", c.Now%1000)
	fmt.Printf("step 3: a naive worker would make ...%03d | 7 | 0, older than the last id\n", c.Now%1000)
	id := w.Next()
	fmt.Printf("step 4-5: Snowflake waited %d ms, clock ...%03d, id %s\n", w.Waited, c.Now%1000, parts(id))
	x := NewWorker(7, &Clock{Now: t123 + 1})
	y := NewWorker(7, &Clock{Now: t123 + 1})
	p, q := x.Next(), y.Next()
	fmt.Printf("step 6: two workers with machine 7: %s and %s, duplicate: %v\n", parts(p), parts(q), p == q)
}
