// Little's law: the story of the lesson 12-littles-law.html.
// An MCP proxy sends 100 agent queries a second to a graph database through a
// pool of 30 connections. L = lambda x W says how many are in flight. When W
// grows and arrivals do not, the queue runs away unless it is capped.
package main

import "fmt"

// InFlight is Little's law: items in the system = arrival rate x time in it.
func InFlight(lambda, w float64) float64 { return lambda * w }

// MaxRate is the law turned round: a pool of slots, each held for w seconds,
// can serve at most slots / w requests a second.
func MaxRate(slots int, w float64) float64 { return float64(slots) / w }

// Proxy holds the pool and its queue. Cap < 0 means the queue has no cap.
type Proxy struct {
	Pool    int
	Cap     int // most queries allowed to wait for a connection
	Waiting int
	Refused int
}

// Second plays one second: lambda queries arrive, the pool serves what it can.
func (p *Proxy) Second(lambda int, w float64) {
	served := int(MaxRate(p.Pool, w))
	p.Waiting += lambda - served // arrivals beyond what the pool serves wait
	if p.Waiting < 0 {
		p.Waiting = 0
	}
	if p.Cap >= 0 && p.Waiting > p.Cap {
		p.Refused += p.Waiting - p.Cap // refuse fast with a clear error
		p.Waiting = p.Cap
	}
}

func main() {
	fmt.Println("== The law")
	l := InFlight(100, 0.2)
	fmt.Printf("L = 100 per second x 0.2 s = %.0f in flight, pool 30: %.0f spare\n", l, 30-l)
	fmt.Printf("backwards: 50 blocks per second x 2 s = %.0f writers needed\n", InFlight(50, 2))
	fmt.Printf("most the pool serves: 30 / 0.2 s = %.0f per second\n", MaxRate(30, 0.2))

	fmt.Println("== W grows to 0.5 s")
	l = InFlight(100, 0.5)
	fmt.Printf("L = 100 x 0.5 = %.0f: 30 hold a connection, %.0f wait\n", l, l-30)
	fmt.Printf("the pool serves 30 / 0.5 s = %.0f per second, 100 arrive\n", MaxRate(30, 0.5))
	p := &Proxy{Pool: 30, Cap: -1, Waiting: 20}
	for s := 1; s <= 3; s++ {
		p.Second(100, 0.5)
		fmt.Printf("no cap, after %d s: %d waiting\n", s, p.Waiting)
	}
	c := &Proxy{Pool: 30, Cap: 10, Waiting: 0}
	for s := 1; s <= 3; s++ {
		c.Second(100, 0.5)
		fmt.Printf("cap 10, after %d s: %d waiting, %d refused in all\n", s, c.Waiting, c.Refused)
	}
}
