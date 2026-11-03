// Replays the at-least-once lesson: message 7 adds 50 to wallet W, the
// consumer crashes before the ack, and the queue delivers message 7 again.
// A naive consumer adds twice. An idempotent one records the id and skips.
package main

import (
	"errors"
	"fmt"
)

type Msg struct {
	ID     int
	Wallet string
	Amount int
}

// Queue keeps a message until it hears an ack.
type Queue struct {
	msgs  []Msg
	acked map[int]bool
}

func (q *Queue) deliver() (Msg, bool) {
	for _, m := range q.msgs {
		if !q.acked[m.ID] { // no ack yet: deliver it (again)
			return m, true
		}
	}
	return Msg{}, false
}

func (q *Queue) ack(m Msg) { q.acked[m.ID] = true }

// Store holds the balances and the ids already applied.
type Store struct {
	balance map[string]int
	seen    map[int]bool
}

// begin copies the store. commit swaps the copy in, so both changes land or none.
func (s *Store) begin() *Store {
	tx := &Store{balance: map[string]int{}, seen: map[int]bool{}}
	for k, v := range s.balance {
		tx.balance[k] = v
	}
	for k, v := range s.seen {
		tx.seen[k] = v
	}
	return tx
}

func (s *Store) commit(tx *Store) { s.balance, s.seen = tx.balance, tx.seen }

var errCrash = errors.New("crashed before the ack")

// consumeNaive adds the amount on every delivery.
func consumeNaive(q *Queue, s *Store, m Msg, crash bool) error {
	s.balance[m.Wallet] += m.Amount // adds every time
	if crash {
		return errCrash // the ack is never sent
	}
	q.ack(m)
	return nil
}

// consumeOnce applies the effect and records the id in one transaction.
func consumeOnce(q *Queue, s *Store, m Msg, crash bool) (applied bool, err error) {
	tx := s.begin()
	if tx.seen[m.ID] { // a repeat: skip the work, just ack
		q.ack(m)
		return false, nil
	}
	tx.balance[m.Wallet] += m.Amount
	tx.seen[m.ID] = true
	s.commit(tx) // the effect and the id commit together
	if crash {
		return true, errCrash
	}
	q.ack(m)
	return true, nil
}

// setBalance sets the value the message carries: twice is the same as once.
func setBalance(s *Store, wallet string, v int) { s.balance[wallet] = v }

func newRun() (*Queue, *Store) {
	q := &Queue{msgs: []Msg{{ID: 7, Wallet: "W", Amount: 50}}, acked: map[int]bool{}}
	s := &Store{balance: map[string]int{"W": 100}, seen: map[int]bool{}}
	return q, s
}

func main() {
	fmt.Println("tab 1: add the amount (naive consumer)")
	q, s := newRun()
	m, _ := q.deliver()
	fmt.Printf("  deliver msg %d: +%d to %s, W holds %d\n", m.ID, m.Amount, m.Wallet, s.balance["W"])
	err := consumeNaive(q, s, m, true)
	fmt.Printf("  consumer adds %d: W holds %d, then %v\n", m.Amount, s.balance["W"], err)
	m, ok := q.deliver()
	fmt.Printf("  queue has no ack for msg %d, delivers it again: %v\n", m.ID, ok)
	_ = consumeNaive(q, s, m, false)
	fmt.Printf("  consumer adds %d again: W holds %d, should be 150, wrong by %d\n", m.Amount, s.balance["W"], s.balance["W"]-150)

	fmt.Println("tab 2: merge by message id (idempotent consumer)")
	q, s = newRun()
	m, _ = q.deliver()
	applied, err := consumeOnce(q, s, m, true)
	fmt.Printf("  msg %d applied=%v: W holds %d, seen ids %v, then %v\n", m.ID, applied, s.balance["W"], s.seen, err)
	m, ok = q.deliver()
	fmt.Printf("  queue delivers msg %d again: %v\n", m.ID, ok)
	applied, _ = consumeOnce(q, s, m, false)
	_, more := q.deliver()
	fmt.Printf("  msg %d applied=%v (id seen, skipped), acked: %v, W holds %d\n", m.ID, applied, !more, s.balance["W"])

	fmt.Println("other way: set the value instead of adding")
	q, s = newRun()
	setBalance(s, "W", 150)
	setBalance(s, "W", 150) // a repeat changes nothing
	fmt.Printf("  set W = 150 twice: W holds %d\n", s.balance["W"])
}
