// Dispatchers: N threads run many actors in turns. Goroutine workers stand in for threads.
// Time is scaled: 100 ms in the program stands for 1 s in the story.
package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

const second = 100 * time.Millisecond // one story second

type Actor struct {
	mailbox   chan func()
	scheduled atomic.Bool
	turns     atomic.Int64
	d         *Dispatcher
}

// Dispatcher: a run queue and a fixed number of workers.
type Dispatcher struct {
	runQ       chan *Actor
	throughput int
}

func newDispatcher(threads, throughput int) *Dispatcher {
	d := &Dispatcher{runQ: make(chan *Actor, 20000), throughput: throughput}
	for i := 0; i < threads; i++ {
		go func() {
			for a := range d.runQ {
				a.runBatch()
			}
		}()
	}
	return d
}

func (d *Dispatcher) spawn() *Actor { return &Actor{mailbox: make(chan func(), 64), d: d} }

// Tell puts the message in the mailbox and asks for a turn if the actor has none.
func (a *Actor) Tell(m func()) {
	a.mailbox <- m
	if a.scheduled.CompareAndSwap(false, true) {
		a.d.runQ <- a
	}
}

// runBatch runs at most throughput messages, then gives the thread back.
func (a *Actor) runBatch() {
	a.turns.Add(1)
	for i := 0; i < a.d.throughput; i++ {
		select {
		case m := <-a.mailbox:
			m()
		default:
			i = a.d.throughput
		}
	}
	a.scheduled.Store(false)
	if len(a.mailbox) > 0 && a.scheduled.CompareAndSwap(false, true) {
		a.d.runQ <- a // still has mail: back in the line
	}
}

func story(d time.Duration) float64 {
	return float64(d.Round(second/10)) / float64(second)
}

// burst sends one blocking 2 s call to each of 8 legacy actors, then times an alarm.
func burst(legacy, def *Dispatcher) (alarm, legacyDone float64) {
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < 8; i++ {
		wg.Add(1)
		legacy.spawn().Tell(func() {
			time.Sleep(2 * second) // a synchronous driver call
			wg.Done()
		})
	}
	time.Sleep(second / 10) // the burst has landed
	got := make(chan time.Duration, 1)
	sent := time.Now()
	def.spawn().Tell(func() { got <- time.Since(sent) }) // the overheat alarm
	alarm = story(<-got)
	wg.Wait()
	return alarm, story(time.Since(start))
}

func main() {
	fmt.Println("== idle actors cost no thread")
	shared := newDispatcher(8, 5)
	before := runtime.NumGoroutine()
	actors := make([]*Actor, 10000)
	for i := range actors {
		actors[i] = shared.spawn()
	}
	fmt.Printf("  10,000 actors exist, goroutines %d before, %d after\n", before, runtime.NumGoroutine())

	fmt.Println("== actors take turns: 20 messages, a batch of 5, then yield")
	var wg sync.WaitGroup
	wg.Add(20)
	for i := 0; i < 20; i++ {
		actors[1].Tell(func() { wg.Done() })
	}
	wg.Wait()
	time.Sleep(second / 10)
	fmt.Printf("  device 1 needed at least %d turns (got %d) on 8 threads\n", 20/5, actors[1].turns.Load())

	fmt.Println("== eight blocking calls on the shared pool")
	alarm, done := burst(shared, shared)
	fmt.Printf("  the overheat alarm waited %.1f s, legacy calls done at %.1f s\n", alarm, done)

	fmt.Println("== a bulkhead: blocking calls on their own pool of 2 threads")
	blocking := newDispatcher(2, 1)
	alarm, done = burst(blocking, shared)
	fmt.Printf("  the overheat alarm waited %.1f s, legacy calls done at %.1f s\n", alarm, done)

	fmt.Println("== do not block at all: start the call, pipe the result back")
	var async sync.WaitGroup
	start := time.Now()
	for i := 0; i < 8; i++ {
		async.Add(1)
		a := shared.spawn()
		a.Tell(func() { // the handler ends at once, no thread waits
			go func() {
				time.Sleep(2 * second) // the driver's async call
				a.Tell(func() { async.Done() })
			}()
		})
	}
	got := make(chan time.Duration, 1)
	sent := time.Now()
	shared.spawn().Tell(func() { got <- time.Since(sent) })
	fmt.Printf("  the overheat alarm waited %.1f s", story(<-got))
	async.Wait()
	fmt.Printf(", 8 calls done at %.1f s, no thread held\n", story(time.Since(start)))
}
