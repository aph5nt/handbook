// Routers: round robin needs only a counter, smallest mailbox looks at queue lengths,
// and consistent hashing gives every key one fixed worker.
package main

import (
	"fmt"
	"hash/crc32"
	"sort"
	"sync"
	"time"
)

// Worker is an actor: a goroutine that ranges over its mailbox channel.
type Worker struct {
	name    string
	mailbox chan func()
}

func spawn(name string) *Worker {
	w := &Worker{name: name, mailbox: make(chan func(), 64)}
	go func() {
		for job := range w.mailbox { // one at a time
			job()
		}
	}()
	return w
}

// RoundRobin only counts. It never looks at the message or the load.
type RoundRobin struct {
	workers []*Worker
	next    int
}

func (r *RoundRobin) Pick() *Worker {
	w := r.workers[r.next%len(r.workers)]
	r.next++
	return w
}

// smallest picks the worker with the shortest queue.
func smallest(ws []*Worker) *Worker {
	best := ws[0]
	for _, w := range ws[1:] {
		if len(w.mailbox) < len(best.mailbox) {
			best = w
		}
	}
	return best
}

// Ring is a consistent hash ring. Each worker owns 205 points on it.
type point struct {
	hash   uint32
	worker string
}
type Ring []point

func newRing(workers ...string) Ring {
	var r Ring
	for _, w := range workers {
		for i := 0; i < 205; i++ {
			r = append(r, point{crc32.ChecksumIEEE([]byte(fmt.Sprintf("worker %s #%d", w, i))), w})
		}
	}
	sort.Slice(r, func(a, b int) bool { return r[a].hash < r[b].hash })
	return r
}

func (r Ring) Lookup(device int) string {
	h := crc32.ChecksumIEEE([]byte(fmt.Sprintf("device-%d", device)))
	i := sort.Search(len(r), func(i int) bool { return r[i].hash >= h }) // first point clockwise
	return r[i%len(r)].worker
}

func roundRobinPackets() {
	fmt.Println("== round robin over packets")
	ws := []*Worker{spawn("parser 1"), spawn("parser 2"), spawn("parser 3")}
	rr := &RoundRobin{workers: ws}
	for p := 1; p <= 4; p++ {
		fmt.Printf("packet %d -> %s\n", p, rr.Pick().name)
	}
}

func slowWorker(pick func(ws []*Worker, rr *RoundRobin) *Worker, label string) {
	ws := []*Worker{spawn("parser 1"), spawn("parser 2"), spawn("parser 3")}
	rr := &RoundRobin{workers: ws}
	release := make(chan struct{})
	ws[1].mailbox <- func() { <-release } // parser 2 is stuck on a huge packet
	rr.next = 2                           // packet 2 went to parser 2, the next is parser 3
	var done sync.WaitGroup
	for p := 3; p <= 14; p++ {
		done.Add(1)
		pick(ws, rr).mailbox <- func() { done.Done() }
		time.Sleep(2 * time.Millisecond) // the quick parsers drain
	}
	fmt.Printf("%s: parser 2 has %d packets waiting behind the huge one\n", label, len(ws[1].mailbox))
	close(release)
	done.Wait()
}

func main() {
	roundRobinPackets()

	fmt.Println("\n== a slow worker, 14 packets")
	slowWorker(func(ws []*Worker, rr *RoundRobin) *Worker { return rr.Pick() }, "round robin")
	slowWorker(func(ws []*Worker, rr *RoundRobin) *Worker { return smallest(ws) }, "smallest mailbox")

	fmt.Println("\n== round robin over readings of one device")
	var mu sync.Mutex
	store := 21.3
	count := map[string]int{}
	a, b := spawn("A"), spawn("B")
	var wg sync.WaitGroup
	wg.Add(2)
	apply := func(w string, c float64, d time.Duration) func() {
		return func() {
			defer wg.Done()
			time.Sleep(d)
			mu.Lock()
			defer mu.Unlock()
			store = c
			count[w]++
			fmt.Printf("worker %s stores %.1f C\n", w, c)
		}
	}
	a.mailbox <- apply("A", 21.5, 60*time.Millisecond) // reading 1, worker A is slow
	b.mailbox <- apply("B", 21.7, 5*time.Millisecond)  // reading 2, worker B is quick
	wg.Wait()
	fmt.Printf("store: %.1f C, the older reading. A knows %d reading, B knows %d\n", store, count["A"], count["B"])

	fmt.Println("\n== consistent hashing by device id")
	ring := newRing("A", "B", "C")
	for _, d := range []int{42, 42, 17, 8} {
		fmt.Printf("device %d -> worker %s\n", d, ring.Lookup(d))
	}
	workers := map[string]*Worker{"A": spawn("A"), "B": spawn("B"), "C": spawn("C")}
	wg.Add(2)
	seen := 0
	for _, c := range []float64{21.5, 21.7} {
		v := c
		workers[ring.Lookup(42)].mailbox <- func() { // one mailbox, in order
			defer wg.Done()
			store, seen = v, seen+1
		}
	}
	wg.Wait()
	fmt.Printf("worker B stored in order: %.1f C, knows %d readings\n", store, seen)

	fmt.Println("\n== the pool changes: worker B leaves")
	smaller := newRing("A", "C")
	fmt.Printf("device 42 -> worker %s, which has no state for it\n", smaller.Lookup(42))
	moved, movedMod := 0, 0
	for d := 0; d < 10000; d++ {
		if ring.Lookup(d) != smaller.Lookup(d) {
			moved++
		}
		if d%3 != d%2 {
			movedMod++
		}
	}
	fmt.Printf("of 10,000 devices: ring moves %d, plain modulo moves %d\n", moved, movedMod)
}
