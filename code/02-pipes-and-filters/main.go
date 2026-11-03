// Replays the pipes and filters lesson: five indexer stages joined by
// channels. With bounded channels a stuck price stage makes fetch wait and
// memory stays flat. With one unbounded channel the queue grows until the
// process is killed.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const (
	firstBlock = 9401
	blocks     = 40 // blocks to index
	capacity   = 3  // slots in each bounded channel
	memBudget  = 20 // swaps the process can hold in a queue before the OS kills it
)

type Item struct{ Block int }

type Pipeline struct {
	ch      [5]chan Item  // ch[1]..ch[4]: the four channels between stages
	fetched atomic.Int64  // blocks fetch has read
	written atomic.Int64  // rows written to the warehouse
	release chan struct{} // closed when the price lookup recovers
	killed  chan int      // receives the queue length when memory runs out
}

// stage is one filter: read one channel, do one job, write the next.
// A send on a full channel blocks. That block is the back-pressure.
func stage(in <-chan Item, out chan<- Item, job func(Item) Item) {
	for it := range in {
		out <- job(it)
	}
	close(out)
}

// unbounded stands in for a channel with no limit: it accepts every item at
// once and keeps it in a growing slice.
func unbounded(in <-chan Item, out chan<- Item, killed chan<- int) {
	var queue []Item
	for in != nil || len(queue) > 0 {
		var send chan<- Item
		var next Item
		if len(queue) > 0 {
			send, next = out, queue[0]
		}
		select {
		case it, ok := <-in:
			if !ok {
				in = nil
				continue
			}
			queue = append(queue, it) // never waits
			if len(queue) > memBudget {
				killed <- len(queue) // out of memory
				return
			}
		case send <- next:
			queue = queue[1:]
		}
	}
	close(out)
}

// decode and extract are fast filters. In this story each block holds one swap.
func decode(it Item) Item  { return it }
func extract(it Item) Item { return it }

func newPipeline(bounded3 bool) *Pipeline {
	p := &Pipeline{release: make(chan struct{}), killed: make(chan int, 1)}
	for k := 1; k <= 4; k++ {
		p.ch[k] = make(chan Item, capacity) // make(chan Item, 3)
	}
	go func() { // stage 1: fetch block
		for b := firstBlock; b < firstBlock+blocks; b++ {
			p.fetched.Add(1)
			p.ch[1] <- Item{Block: b} // waits when channel 1 is full
		}
		close(p.ch[1])
	}()
	go stage(p.ch[1], p.ch[2], decode) // stage 2
	if bounded3 {
		go stage(p.ch[2], p.ch[3], extract) // stage 3
	} else {
		p.ch[3] = make(chan Item) // the queue hands swaps to pricing one at a time
		raw := make(chan Item)    // extract hands each swap straight to the queue
		go stage(p.ch[2], raw, extract)
		go unbounded(raw, p.ch[3], p.killed)
	}
	go func() { // stage 5: write rows
		for range p.ch[4] {
			p.written.Add(1)
		}
	}()
	return p
}

// price is stage 4. The lookup hangs until release is closed.
func (p *Pipeline) price(in <-chan Item, out chan<- Item, wg *sync.WaitGroup) {
	defer wg.Done()
	for it := range in {
		<-p.release // the price lookup is stuck
		out <- it
	}
}

// settle waits until nothing moves any more and returns the channel lengths.
func (p *Pipeline) settle() [5]int {
	var last [5]int
	same := 0
	for same < 5 {
		time.Sleep(20 * time.Millisecond)
		var now [5]int
		now[0] = int(p.fetched.Load())
		for k := 1; k <= 4; k++ {
			now[k] = len(p.ch[k])
		}
		if now == last {
			same++
		} else {
			same, last = 0, now
		}
	}
	return last
}

func bounded() {
	fmt.Println("tab 1: bounded channels, capacity 3 each")
	p := newPipeline(true)
	var wg sync.WaitGroup
	wg.Add(1)
	go p.price(p.ch[3], p.ch[4], &wg) // copy 1 of the price stage
	s := p.settle()
	waiting := s[1] + s[2] + s[3] + s[4]
	fmt.Printf("  price lookup stuck: fetch stopped after %d of %d blocks\n", s[0], blocks)
	fmt.Printf("  channel lengths 1..4: %d %d %d %d\n", s[1], s[2], s[3], s[4])
	fmt.Printf("  items waiting in channels: %d (bound: 4 x %d = %d)\n", waiting, capacity, 4*capacity)

	wg.Add(1)
	go p.price(p.ch[3], p.ch[4], &wg) // copy 2 reads the same channel
	close(p.release)                  // the lookup recovers
	go func() { wg.Wait(); close(p.ch[4]) }()
	for p.written.Load() < blocks {
		time.Sleep(5 * time.Millisecond)
	}
	fmt.Printf("  with a second price copy: written %d of %d\n", p.written.Load(), blocks)
}

func unboundedRun() {
	fmt.Printf("tab 2: channel 3 has no limit, memory budget %d queued swaps\n", memBudget)
	p := newPipeline(false)
	var wg sync.WaitGroup
	wg.Add(1)
	go p.price(p.ch[3], p.ch[4], &wg)
	n := <-p.killed
	fmt.Printf("  queue in front of pricing reached %d swaps: process killed\n", n)
	fmt.Printf("  swaps lost: %d (the queue plus the one pricing held), rows written: %d\n", n+1, p.written.Load())
	fmt.Printf("  restart from the last checkpoint: block %d\n", firstBlock)
}

func main() {
	bounded()
	unboundedRun()
	fmt.Println("fix: channel 3 bounded at 3 again")
	p := newPipeline(true)
	var wg sync.WaitGroup
	wg.Add(1)
	go p.price(p.ch[3], p.ch[4], &wg)
	s := p.settle()
	fmt.Printf("  price lookup stuck: fetch waits after %d blocks, %d items in channels\n", s[0], s[1]+s[2]+s[3]+s[4])
}
