// Session guarantees: an order page reads from two replicas. Replica 2 is one
// log position behind. Without a session check order 5 vanishes on refresh.
// A sticky session fixes it until the pinned replica dies. A log position the
// client carries fixes it for good.
package main

import "fmt"

// Replica holds the orders up to the log position it has applied.
type Replica struct {
	name string
	pos  int
	dead bool
}

// orders at log position p: order n was written at position 36+n,
// so position 40 holds 4 orders and position 41 holds 5.
func ordersAt(p int) int { return p - 36 }

// Client is one customer's page. keep says whether it keeps the position.
type Client struct {
	seen int
	keep bool
}

// Balancer picks a replica: round robin, or the one pinned to the client.
type Balancer struct {
	replicas []*Replica
	next     int
	sticky   *Replica
}

func (lb *Balancer) Pick() *Replica {
	if lb.sticky != nil {
		return lb.sticky // the replica pinned to this client
	}
	r := lb.replicas[lb.next%len(lb.replicas)]
	lb.next++ // round robin: knows nothing about the client
	return r
}

// Read runs one page load. catchUp lets a lagging replica apply the log.
func Read(lb *Balancer, c *Client, catchUp func(*Replica)) (int, string) {
	r := lb.Pick()
	if r.dead {
		r = lb.replicas[1] // the sticky replica died: fall back
	}
	if r.pos < c.seen {
		catchUp(r) // wait until the replica has reached what we saw
	}
	rows := ordersAt(r.pos)
	if c.keep && r.pos > c.seen {
		c.seen = r.pos // remember the newest position seen
	}
	return rows, r.name
}

func main() {
	catchUp := func(r *Replica) { r.pos = 41 }
	noWait := func(*Replica) {}

	fmt.Println("== No session check")
	r1, r2 := &Replica{"replica 1", 41, false}, &Replica{"replica 2", 40, false}
	lb := &Balancer{replicas: []*Replica{r1, r2}}
	c := &Client{keep: false}
	n, from := Read(lb, c, noWait)
	fmt.Printf("read: %s at pos %d shows %d orders\n", from, r1.pos, n)
	n, from = Read(lb, c, noWait)
	fmt.Printf("refresh: %s at pos %d shows %d orders: order 5 vanished\n", from, r2.pos, n)

	fmt.Println("== Sticky session")
	r1, r2 = &Replica{"replica 1", 41, false}, &Replica{"replica 2", 40, false}
	lb = &Balancer{replicas: []*Replica{r1, r2}, sticky: r1}
	c = &Client{keep: false}
	n, from = Read(lb, c, noWait)
	fmt.Printf("refresh: %s shows %d orders\n", from, n)
	r1.dead = true
	n, from = Read(lb, c, noWait)
	fmt.Printf("replica 1 dies: %s shows %d orders: vanished again\n", from, n)

	fmt.Println("== Client carries its position")
	r1, r2 = &Replica{"replica 1", 41, false}, &Replica{"replica 2", 40, false}
	lb = &Balancer{replicas: []*Replica{r1, r2}, sticky: r1}
	c = &Client{keep: true}
	n, _ = Read(lb, c, catchUp)
	fmt.Printf("read: %d orders, page keeps seen = %d\n", n, c.seen)
	r1.dead = true
	before := r2.pos
	n, from = Read(lb, c, catchUp)
	fmt.Printf("replica 1 dies: %s was at %d < seen %d, waits, reaches %d, shows %d orders\n", from, before, c.seen, r2.pos, n)
}
