// Message time to live: a request nobody waits for is dropped, and a
// queue-wide time to live on commands silently drops orders.
//
// A Go struct stands in for a RabbitMQ queue. Expiry is checked only at the
// head, as RabbitMQ does for per-message expiration. Time is in milliseconds
// and just added up, so nothing sleeps. Deterministic.
package main

import "fmt"

type Msg struct {
	ID      string
	Expires int // absolute time, 0 = never
}

type Queue struct {
	msgs    []Msg
	dlx     *[]Msg // dead-letter target, nil = dropped for good
	dropped int
}

// Next returns the next live message. Expired heads are removed first.
func (q *Queue) Next(now int) (Msg, bool) {
	for len(q.msgs) > 0 {
		m := q.msgs[0]
		q.msgs = q.msgs[1:]
		if m.Expires != 0 && now >= m.Expires {
			q.dropped++
			if q.dlx != nil {
				*q.dlx = append(*q.dlx, m)
			}
			continue // the worker never sees it
		}
		return m, true
	}
	return Msg{}, false
}

func main() {
	fmt.Println("== Tab 1: expiry on a request ==")
	q := &Queue{}
	sentAt := 0
	q.msgs = append(q.msgs, Msg{"GetPrice q-41", sentAt + 2000}) // caller waits 2 s
	q.msgs = append(q.msgs, Msg{"GetPrice q-99", 4000 + 2000})
	now := 4000 // workers were backed up for 4 s
	fmt.Printf("t=%d ms API gave up on q-41 at 2000 ms\n", now)
	m, _ := q.Next(now)
	fmt.Printf("t=%d ms broker dropped %d expired, worker gets %s\n", now, q.dropped, m.ID)
	worked := 0
	stale := []int{2000, 2000, 2000} // without expiry: three stale requests computed
	for range stale {
		worked++
	}
	fmt.Printf("without a time to live: the worker computes %d answers nobody reads\n", worked)

	fmt.Println("== Tab 2: expiry on a command ==")
	const ttl = 60_000
	orders := &Queue{}
	for i := 501; i <= 505; i++ {
		orders.msgs = append(orders.msgs, Msg{fmt.Sprintf("ChargeCard %d", i), (i-501)*10_000 + ttl})
	}
	_, ok := orders.Next(5 * 60_000) // workers back after 5 minutes
	fmt.Printf("workers back at 5 min: got a message=%v, dropped %d orders, no error logged\n", ok, orders.dropped)

	var expired []Msg
	fix := &Queue{dlx: &expired}
	for i := 501; i <= 505; i++ {
		fix.msgs = append(fix.msgs, Msg{fmt.Sprintf("ChargeCard %d", i), (i-501)*10_000 + ttl})
	}
	fix.Next(5 * 60_000)
	fmt.Printf("with a dead-letter exchange: payments.expired holds %d, the alert fires\n", len(expired))

	noTTL := &Queue{}
	noTTL.msgs = append(noTTL.msgs, Msg{"ChargeCard 501", 0})
	m, _ = noTTL.Next(5 * 60_000)
	fmt.Printf("with no time to live: after the deploy the worker gets %s\n", m.ID)
}
