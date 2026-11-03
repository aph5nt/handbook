// Mailbox overflow: 1,000 readings a second in, 200 a second out. A plain tell stores the
// difference in memory. A bound caps it. A stream with demand never lets it build up.
// Time is simulated: one tick of the loop is one second of the story.
package main

import (
	"fmt"
	"time"
)

const in, out = 1000, 200

type Reading struct{}

// writer is the actor: each tick it takes what one second of writes can take.
func writer(mailbox chan Reading, tick chan struct{}, ack chan struct{}) {
	go func() {
		for range tick {
			for i := 0; i < out && len(mailbox) > 0; i++ {
				<-mailbox
			}
			ack <- struct{}{}
		}
	}()
}

func plain() {
	fmt.Println("== plain tell, no limit: depth = (1,000 - 200) x seconds")
	for _, t := range []int{1, 60, 3600} {
		fmt.Printf("  after %4d s: %d waiting\n", t, (in-out)*t)
	}
	fmt.Println("  the heap fills, the process dies, and every queued reading is lost")
}

func bounded() {
	const capacity = 10000
	fmt.Printf("== bounded mailbox, capacity %d, drop the newest\n", capacity)
	mailbox := make(chan Reading, capacity)
	tick, ack := make(chan struct{}), make(chan struct{})
	writer(mailbox, tick, ack)
	dead, last := 0, 0
	for t := 1; t <= 20; t++ {
		for i := 0; i < in; i++ {
			select {
			case mailbox <- Reading{}:
			default:
				dead++ // full: dead letters, counted
			}
		}
		if t == 13 || t == 14 || t == 20 {
			fmt.Printf("  t=%2d s: depth %d, dead letters %d (+%d)\n", t, len(mailbox), dead, dead-last)
		}
		last = dead
		tick <- struct{}{}
		<-ack
	}
	fmt.Printf("  it fills after %d / %d = %.1f s, then 800 a second are dropped\n", capacity, in-out, float64(capacity)/float64(in-out))
}

func blocking() {
	fmt.Println("== the other rule: block the sender")
	mailbox := make(chan Reading, 1) // full, and nobody reads it for now
	mailbox <- Reading{}
	pool := make(chan struct{}, 1) // a dispatcher with one thread
	pool <- struct{}{}             // the sender holds the thread
	go func() {
		select {
		case mailbox <- Reading{}: // blocked until there is room
		case <-time.After(50 * time.Millisecond): // the push timeout
		}
		<-pool
	}()
	start := time.Now()
	pool <- struct{}{} // another actor on the same dispatcher wants the thread
	fmt.Printf("  another actor waited about %v for the thread, for one reading\n", time.Since(start).Round(10*time.Millisecond))
}

// stream runs 60 s with demand: the writer requests, the source reads only that much.
func stream() {
	fmt.Println("== stream: the writer pulls")
	demand := make(chan int)
	data := make(chan Reading, out)
	sent := 0
	go func() {
		for n := range demand { // only what was asked
			for i := 0; i < n; i++ {
				data <- Reading{}
				sent++
			}
		}
	}()
	for t := 1; t <= 60; t++ {
		demand <- out // the writer asks for 200
		for i := 0; i < out; i++ {
			<-data
		}
	}
	fmt.Printf("  after 60 s: read from the socket %d, buffer capacity %d, so the heap stays flat\n", sent, cap(data))
	fmt.Printf("  the device holds back %d readings on its side, slowed by TCP\n", (in*60)-sent)
}

// tellInTheMiddle pulls at full speed and hands on with a plain tell.
func tellInTheMiddle() {
	fmt.Println("== a stage with a plain tell in the middle")
	mailbox := make(chan Reading, in*60)
	tick, ack := make(chan struct{}), make(chan struct{})
	writer(mailbox, tick, ack)
	for t := 1; t <= 60; t++ {
		for i := 0; i < in; i++ { // no demand signal: the stage always wants more
			mailbox <- Reading{} // tell
		}
		tick <- struct{}{}
		<-ack
	}
	fmt.Printf("  after 60 s: actor mailbox %d, growing again\n", len(mailbox))
}

// tellWithAck is the fix: the stage waits for the actor's ack before the next item.
func tellWithAck() {
	fmt.Println("== the fix: the stage waits for an ack")
	mailbox, ack := make(chan Reading, in), make(chan struct{})
	maxDepth, handed := 0, 0
	go func() {
		for range mailbox {
			ack <- struct{}{} // done, send the next
		}
	}()
	for i := 0; i < out*60; i++ { // the actor sets the pace: 200 a second for 60 s
		mailbox <- Reading{}
		if len(mailbox) > maxDepth {
			maxDepth = len(mailbox)
		}
		<-ack
		handed++
	}
	fmt.Printf("  after 60 s: %d handed on, actor mailbox never above %d\n", handed, maxDepth)
}

func main() {
	plain()
	bounded()
	blocking()
	stream()
	tellInTheMiddle()
	tellWithAck()
}
