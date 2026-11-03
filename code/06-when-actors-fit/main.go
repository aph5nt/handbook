// When actors fit: three questions decide. Then each shape is run: per-device actors,
// a pipeline with bounded channels, a writer actor with no bound, and a report that
// asks 1,000 actors against one query on a read model.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Workload struct {
	Name                             string
	ManyThingsWithState, NaturalKey  bool
	MostQueriesTouchOne, CrossesMany bool
}

// decide walks the tree. Each question ends the walk on a "no".
func decide(w Workload) string {
	if !w.ManyThingsWithState {
		return "not actors: a pipeline, a pool or a service"
	}
	if !w.NaturalKey {
		return "not actors: no key for a message to find its actor"
	}
	if !w.MostQueriesTouchOne {
		return "not actors: a database"
	}
	if w.CrossesMany {
		return "actors, plus a read model for the cross-device query"
	}
	return "actors"
}

// pipeline: decode pushes into a channel of size cap, the writer takes 1 ms per batch.
// It returns the peak queue length seen by the sender.
func pipeline(batches, size int) int {
	ch := make(chan int, size)
	var peak atomic.Int64
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range ch {
			time.Sleep(time.Millisecond) // StarRocks is slow
		}
	}()
	for i := 0; i < batches; i++ {
		ch <- i // a full channel makes decode wait: back-pressure
		if n := int64(len(ch)); n > peak.Load() {
			peak.Store(n)
		}
	}
	close(ch)
	wg.Wait()
	return int(peak.Load())
}

type Ask struct{ Reply chan float64 }

// device is an actor. A busy device has a backlog ahead of every ask.
func device(c float64, backlog time.Duration, store func(float64)) chan Ask {
	mb := make(chan Ask, 1)
	go func() {
		for a := range mb {
			time.Sleep(backlog)
			a.Reply <- c
		}
	}()
	if store != nil {
		store(c) // each actor writes its own state to the read model
	}
	return mb
}

func main() {
	fmt.Println("== the three questions")
	for _, w := range []Workload{
		{"IoT device state", true, true, true, false},
		{"IoT, building average too", true, true, true, true},
		{"Go indexer: blocks to StarRocks", false, false, false, false},
	} {
		fmt.Printf("  %-32s -> %s\n", w.Name, decide(w))
	}

	fmt.Println("== the pipeline: 500 batches, a writer at 1 ms a batch")
	fmt.Printf("  bounded channel, size 8:  peak queue %d, decode waited\n", pipeline(500, 8))
	fmt.Printf("  writer actor, no bound:   peak queue %d of 500, nothing slowed decode\n", pipeline(500, 1000000))

	fmt.Println("== the report: average of 1,000 devices")
	var mu sync.Mutex
	readModel := map[int]float64{}
	boxes := make([]chan Ask, 1000)
	for i := range boxes {
		backlog := time.Duration(0)
		if i%20 == 0 {
			backlog = 300 * time.Millisecond // 50 busy devices
		}
		id := i
		boxes[i] = device(20+float64(i%5), backlog, func(c float64) {
			mu.Lock()
			readModel[id] = c
			mu.Unlock()
		})
	}
	start := time.Now()
	type answer struct {
		C  float64
		OK bool
	}
	answers := make(chan answer, 1000)
	for _, mb := range boxes { // 1,000 temporary replies and 1,000 timers
		go func(mb chan Ask) {
			reply := make(chan float64, 1)
			mb <- Ask{reply}
			select {
			case c := <-reply:
				answers <- answer{c, true}
			case <-time.After(200 * time.Millisecond):
				answers <- answer{}
			}
		}(mb)
	}
	got, timedOut, sum := 0, 0, 0.0
	for range boxes {
		a := <-answers
		if a.OK {
			got++
			sum += a.C
		} else {
			timedOut++
		}
	}
	fmt.Printf("  ask every actor: %d asks, %d answered, %d timed out, took %d ms, average of a part: %.2f C\n",
		len(boxes), got, timedOut, time.Since(start).Round(100*time.Millisecond).Milliseconds(), sum/float64(got))

	start = time.Now()
	sum = 0
	mu.Lock()
	for _, c := range readModel {
		sum += c
	}
	n := len(readModel)
	mu.Unlock()
	fmt.Printf("  one read-model query: 1 query, %d rows, under %d ms, average %.2f C\n", n, 1+time.Since(start).Milliseconds(), sum/float64(n))
}
