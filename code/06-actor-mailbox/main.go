// Actor and mailbox: three senders, one ordered line, no lock.
// A goroutine that ranges over a channel is the actor. Only it touches its state.
package main

import (
	"fmt"
	"sync"
)

type Msg interface{}

type Reading struct{ C float64 }
type SetInterval struct{ Seconds int }
type ReadingStored struct{ C float64 }
type Batch struct{ Readings []float64 } // a mutable slice: the trap
type Add struct{ N int }
type Get struct{ Reply chan int }

// Actor: a mailbox and one goroutine that reads it.
type Actor struct{ mailbox chan Msg }

func spawn(handle func(m Msg)) *Actor {
	a := &Actor{mailbox: make(chan Msg, 16)}
	go func() {
		for m := range a.mailbox { // one message at a time
			handle(m)
		}
	}()
	return a
}

func (a *Actor) Tell(m Msg) { a.mailbox <- m }

func main() {
	fmt.Println("== one owner, one line")
	var done sync.WaitGroup
	rules := spawn(func(m Msg) { fmt.Printf("rules: got %T %v\n", m, m); done.Done() })

	last, count, interval := 0.0, 0, 60 // device 42's state, owned by its goroutine
	device42 := spawn(func(m Msg) {
		switch v := m.(type) {
		case Reading:
			last, count = v.C, count+1
			fmt.Printf("device 42: last %.1f C, count %d\n", last, count)
			done.Done()
		case SetInterval:
			interval = v.Seconds
			fmt.Printf("device 42: interval %d s, tell rules ReadingStored\n", interval)
			rules.Tell(ReadingStored{last})
		}
	})

	// Three threads send at the same instant. The order is set by arrival.
	// The program fixes the order with a chain so the output matches the lesson.
	done.Add(3)
	step := make(chan struct{})
	go func() { device42.Tell(Reading{21.5}); step <- struct{}{} }()
	<-step
	go func() { device42.Tell(Reading{21.7}); step <- struct{}{} }()
	<-step
	go func() { device42.Tell(SetInterval{30}); step <- struct{}{} }()
	<-step
	done.Wait()
	fmt.Println("no lock was taken: only the device 42 goroutine read or wrote its state")
	device43 := spawn(func(m Msg) { // its own goroutine
		fmt.Println("device 43: handled its own reading on its own goroutine")
		done.Done()
	})
	done.Add(1)
	device43.Tell(Reading{19.0})
	done.Wait()

	fmt.Println("\n== trap 1: a mutable list inside a message")
	list := []float64{21.5, 21.7, 22.0}
	half := make(chan struct{})
	mutated := make(chan struct{})
	sum := make(chan float64)
	summer := spawn(func(m Msg) {
		b := m.(Batch)
		s := 0.0
		for i, r := range b.Readings {
			s += r
			if i == 0 {
				half <- struct{}{} // the sender acts here, half way through
				<-mutated
			}
		}
		sum <- s
	})
	summer.Tell(Batch{list}) // the sender keeps a reference to the same backing array
	<-half
	list[2] = 99.0 // thread 1 changes its list
	mutated <- struct{}{}
	fmt.Printf("sum %.1f C, should be %.1f C\n", <-sum, 21.5+21.7+22.0)

	fmt.Println("\n== trap 2: two actors, one database row")
	row := 40 // the building A total in a shared row
	var bothRead, wg sync.WaitGroup
	bothRead.Add(2)
	for _, name := range []string{"device 42", "device 43"} {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			v := row // both read 40
			bothRead.Done()
			bothRead.Wait() // the race window: both have read, none has written
			row = v + 1
			fmt.Printf("%s: read %d, write %d\n", n, v, v+1)
		}(name)
	}
	wg.Wait()
	fmt.Printf("total = %d, should be 42\n", row)

	fmt.Println("\n== fix 1: the message carries a copy")
	list = []float64{21.5, 21.7, 22.0}
	summer.Tell(Batch{append([]float64(nil), list...)}) // a value nobody else holds
	<-half
	list[2] = 99.0
	mutated <- struct{}{}
	fmt.Printf("sum %.1f C, correct\n", <-sum)

	fmt.Println("\n== fix 2: the total has one owner")
	total := 40
	building := spawn(func(m Msg) {
		switch v := m.(type) {
		case Add:
			total += v.N
		case Get:
			v.Reply <- total
		}
	})
	building.Tell(Add{1}) // device 42
	building.Tell(Add{1}) // device 43
	reply := make(chan int)
	building.Tell(Get{reply})
	fmt.Printf("building actor: total = %d\n", <-reply)
}
