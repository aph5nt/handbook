// Outbox and idempotency: order 501 is saved, published twice, and charged
// once. The program also replays the dual write that loses an event and the
// missing key table that charges twice.
package main

import "fmt"

// Row is one outbox row.
type Row struct {
	id     int
	status string
}

// Store is the order database. PlaceOrder commits both rows or neither.
type Store struct {
	orders []int
	outbox []*Row
}

// PlaceOrder inserts the order and its outbox row in one transaction.
func (s *Store) PlaceOrder(id int) {
	s.orders = append(s.orders, id)
	s.outbox = append(s.outbox, &Row{id, "pending"})
}

// RelayOnce publishes pending rows. crash stops it after a publish, before
// the row is marked sent.
func RelayOnce(s *Store, topic *[]int, crash bool) {
	for _, row := range s.outbox {
		if row.status != "pending" {
			continue
		}
		*topic = append(*topic, row.id) // may be a second copy
		if crash {
			return // the row stays pending
		}
		row.status = "sent"
	}
}

// Payment charges a card. keys is the processed table, nil when there is none.
type Payment struct {
	keys    map[int]bool
	charges int
}

// Handle runs one delivery in one transaction, and returns true when the
// message may be acknowledged.
func (p *Payment) Handle(id int, crashBeforeAck bool) bool {
	if p.keys != nil && p.keys[id] {
		return true // duplicate: skip, and acknowledge
	}
	p.charges++
	if p.keys != nil {
		p.keys[id] = true // commits together with the charge
	}
	return !crashBeforeAck
}

// DualWrite is the broken way: commit, then publish from code.
func DualWrite(s *Store, topic *[]int, id int, crashBetween bool) {
	s.orders = append(s.orders, id)
	if crashBetween {
		return // the event exists nowhere
	}
	*topic = append(*topic, id)
}

func main() {
	fmt.Println("== Outbox and key")
	s := &Store{}
	var topic []int
	s.PlaceOrder(501)
	fmt.Println("orders", s.orders, "outbox", s.outbox[0].id, s.outbox[0].status)
	RelayOnce(s, &topic, true)
	fmt.Println("relay crashed: topic", topic, "outbox", s.outbox[0].status)
	RelayOnce(s, &topic, false)
	fmt.Println("relay restarted: topic", topic, "outbox", s.outbox[0].status)
	p := &Payment{keys: map[int]bool{}}
	for _, id := range topic {
		p.Handle(id, false)
		fmt.Println("delivered", id, "charges", p.charges)
	}

	fmt.Println("== No outbox: dual write")
	s = &Store{}
	topic = nil
	DualWrite(s, &topic, 501, true)
	fmt.Println("crash between: orders", s.orders, "topic", topic, "event lost")
	p = &Payment{keys: map[int]bool{}}
	topic = append(topic, 502) // publish first, then the commit of 502 fails
	p.Handle(502, false)
	fmt.Println("publish first, commit fails: orders", s.orders, "charges", p.charges, "for an order that does not exist")
	s.PlaceOrder(503)
	RelayOnce(s, &topic, false)
	fmt.Println("fix, one transaction: orders", s.orders, "outbox", s.outbox[0].status)

	fmt.Println("== No key: duplicate")
	p = &Payment{}
	acked := p.Handle(501, true)
	fmt.Println("charge, crash before ack: acked", acked, "charges", p.charges)
	p.Handle(501, false)
	fmt.Println("redelivered, no key table: charges", p.charges)
	p = &Payment{keys: map[int]bool{}}
	p.Handle(501, true)
	p.Handle(501, false)
	fmt.Println("fix, key table: charges", p.charges)
}
