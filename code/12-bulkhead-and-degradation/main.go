// Bulkhead and graceful degradation: the story of
// 12-bulkhead-and-degradation.html. A service reads a warehouse and a graph
// database through 20 connections. The graph slows down. One shared pool
// drowns; two pools of 10 keep the warehouse working and the answer says
// what is missing.
package main

import (
	"errors"
	"fmt"
)

// Pool is a fixed set of connections.
type Pool struct {
	Name       string
	Size, Busy int
}

// TryAcquire never waits: a full pool refuses at once.
func (p *Pool) TryAcquire() error {
	if p.Busy >= p.Size {
		return errors.New(p.Name + " pool full: refused fast")
	}
	p.Busy++
	return nil
}

// Acquire waits for a free connection, up to the 5 s timeout.
// Nothing is released in this story, so a full pool always times out.
func (p *Pool) Acquire() error {
	if p.Busy >= p.Size {
		return errors.New("no free connection: waited 5 s, timed out")
	}
	p.Busy++
	return nil
}

// Answer builds the reply. With the graph missing it says so.
func Answer(rows int, graphErr error, silent bool) string {
	if graphErr == nil {
		return fmt.Sprintf("%d rows and graph links", rows)
	}
	if silent {
		return fmt.Sprintf("%d rows, no graph links found", rows) // BUG: reads as a fact
	}
	return fmt.Sprintf("%d rows. Graph links missing: graph is slow", rows)
}

func main() {
	fmt.Println("== One shared pool")
	shared := &Pool{Name: "shared", Size: 20}
	for i := 0; i < 20; i++ {
		shared.Acquire() // slow graph calls arrive, each holds its connection
	}
	fmt.Printf("%d of %d busy, all graph\n", shared.Busy, shared.Size)
	fmt.Println("warehouse call:", shared.Acquire())

	fmt.Println("== Bulkheads: two pools of 10")
	wh := &Pool{Name: "warehouse", Size: 10}
	gr := &Pool{Name: "graph", Size: 10}
	refused := 0
	var graphErr error
	for i := 0; i < 20; i++ {
		if err := gr.TryAcquire(); err != nil {
			refused++
			graphErr = err
		}
	}
	fmt.Printf("graph %d of 10 busy, %d graph calls refused fast\n", gr.Busy, refused)
	for i := 0; i < 5; i++ {
		if err := wh.TryAcquire(); err != nil {
			fmt.Println(err)
		}
	}
	fmt.Printf("warehouse %d of 10 busy: its calls still flow\n", wh.Busy)
	fmt.Println("answer:", Answer(42, graphErr, false))
	fmt.Println("silent version:", Answer(42, graphErr, true))

	busy := &Pool{Name: "warehouse", Size: 10}
	waiting := 0
	for i := 0; i < 25; i++ { // a busy hour needs 25 at once
		if busy.TryAcquire() != nil {
			waiting++
		}
	}
	fmt.Printf("pool sized 10 by guess, busy hour needs 25: %d waiting behind the wall\n", waiting)
}
