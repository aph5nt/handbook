// Reactive Streams: a subscriber asks for n, its publisher sends at most n.
// Stages are goroutines, each link is a channel with a buffer of 16.
// The source is the socket: it reads a reading only when demand reaches it.
package main

import (
	"fmt"
	"sync/atomic"
	"time"
)

const bufferSize = 16

type Reading struct{ N int }

type Pipeline struct {
	demand             chan int // writer -> parse -> source
	toParse, toForward chan Reading
	toWriter           chan Reading
	read, written      atomic.Int64
}

func start() *Pipeline {
	p := &Pipeline{
		demand:    make(chan int, 8),
		toParse:   make(chan Reading, bufferSize),
		toForward: make(chan Reading, bufferSize),
		toWriter:  make(chan Reading, bufferSize),
	}
	upstream := make(chan int, 8)
	go func() { // parse passes the demand on, one stage at a time
		for n := range p.demand {
			upstream <- n
		}
	}()
	go func() { // the socket source reads only what was asked
		for n := range upstream {
			for i := 0; i < n; i++ {
				p.read.Add(1)
				p.toParse <- Reading{i} // blocks when the buffer is full
			}
		}
	}()
	go func() { // the parse stage
		for r := range p.toParse {
			p.toForward <- r
		}
	}()
	go func() { // the writer's input buffer
		for r := range p.toForward {
			p.toWriter <- r
		}
	}()
	return p
}

// batch is the writer: request 100, take 100, write them as one batch.
func (p *Pipeline) batch() {
	p.demand <- 100
	for i := 0; i < 100; i++ {
		<-p.toWriter
	}
	p.written.Add(100)
}

// settle waits until the stages have stopped moving.
func (p *Pipeline) settle() {
	for last := int64(-1); last != p.read.Load(); {
		last = p.read.Load()
		time.Sleep(20 * time.Millisecond)
	}
}

func main() {
	fmt.Println("== demand: request 100, data flows down")
	p := start()
	p.batch()
	p.settle()
	fmt.Printf("  read %d, written %d: no stage sent more than was asked\n", p.read.Load(), p.written.Load())

	fmt.Println("== the warehouse slows: the writer has asked once more, and stops taking")
	p.demand <- 100
	p.settle()
	fmt.Printf("  buffers: %d, %d, %d of %d\n", len(p.toParse), len(p.toForward), len(p.toWriter), bufferSize)
	fmt.Printf("  read %d of the 200 asked: the source stopped reading, TCP slows the devices\n", p.read.Load())

	fmt.Println("== the warehouse recovers")
	for i := 0; i < 100; i++ {
		<-p.toWriter
	}
	p.written.Add(100)
	p.batch()
	p.settle()
	fmt.Printf("  read %d, written %d, nothing lost\n", p.read.Load(), p.written.Load())

	tellInTheMiddle()
	withAck()
	droppingBuffer()
}

// tellInTheMiddle: tells never refuse, so the stage keeps reading at the full rate.
// One tick of the loop is one simulated second.
func tellInTheMiddle() {
	const in = 5000
	fmt.Printf("== a plain tell in the middle, %d readings a second\n", in)
	mailbox := make(chan Reading, 1000000)
	done, tick := make(chan struct{}), make(chan int)
	go func() {
		for n := range tick {
			for i := 0; i < n; i++ {
				<-mailbox
			}
			done <- struct{}{}
		}
	}()
	run := func(seconds, out int) {
		for t := 0; t < seconds; t++ {
			for i := 0; i < in; i++ {
				mailbox <- Reading{i} // tell: always succeeds
			}
			tick <- out
			<-done
		}
	}
	run(5, 4000)
	fmt.Printf("  warehouse fast, 4,000 a second: after 5 s the mailbox holds %d\n", len(mailbox))
	run(40, 100)
	fmt.Printf("  warehouse slow, 100 a second: after 40 s more it holds %d, then out of memory\n", len(mailbox))
}

func withAck() {
	fmt.Println("== the fix: the actor acks each reading")
	mailbox, ack := make(chan Reading, 1000), make(chan struct{})
	go func() {
		for range mailbox {
			ack <- struct{}{}
		}
	}()
	maxDepth := 0
	for i := 0; i < 1000; i++ {
		mailbox <- Reading{i}
		if len(mailbox) > maxDepth {
			maxDepth = len(mailbox)
		}
		<-ack // the ack is the demand signal
	}
	fmt.Printf("  1000 readings, mailbox never above %d\n", maxDepth)
}

func droppingBuffer() {
	const capacity, in, out, seconds = 100000, 5000, 100, 60
	queue, dropped := 0, 0
	for t := 0; t < seconds; t++ {
		queue += in
		if queue > capacity {
			dropped += queue - capacity // drop oldest, silently
			queue = capacity
		}
		queue -= out
	}
	fmt.Println("== a second trap: buffer(100000, drop oldest)")
	fmt.Printf("  after %d s under load: nothing slowed, %d readings silently dropped\n", seconds, dropped)
}
