// Domain events: one transaction changes one aggregate and records the event.
// The other aggregate reacts in its own transaction, a moment later.
package main

import (
	"errors"
	"fmt"
)

// Wallet and Subscription are two aggregates with two borders.
type Wallet struct{ Balance int }
type Subscription struct {
	Price   int
	Paid    bool
	Blocked bool // a nightly job holds the row lock
}

// DB is one database with an outbox table.
type DB struct {
	Wallet Wallet
	Outbox []string
}

// TopUp is transaction 1: state and event are saved together, or not at all.
func TopUp(db *DB, amount int) error {
	if amount <= 0 {
		return errors.New("refused: amount must be positive")
	}
	db.Wallet.Balance += amount
	db.Outbox = append(db.Outbox, "WalletToppedUp")
	return nil // commit
}

// Relay publishes unsent rows. The subscription hears a moment later.
func Relay(db *DB, s *Subscription) {
	for _, ev := range db.Outbox {
		OnEvent(ev, s)
	}
	db.Outbox = nil
}

// OnEvent is transaction 2, in the subscription's own border.
func OnEvent(ev string, s *Subscription) {
	if ev == "WalletToppedUp" {
		s.Paid = true
	}
}

// TopUpAndRenew is failure 1: two aggregates in one transaction.
func TopUpAndRenew(db *DB, s *Subscription, amount int) error {
	if s.Blocked {
		return errors.New("top-up times out: waiting for the Subscription lock")
	}
	db.Wallet.Balance += amount
	s.Paid = true
	return nil
}

// TopUpThenSend is failure 2: the event is sent outside the transaction.
func TopUpThenSend(db *DB, s *Subscription, amount int, crash bool) {
	db.Wallet.Balance += amount // committed
	if crash {
		return // the event is never sent
	}
	OnEvent("WalletToppedUp", s)
}

func main() {
	db := &DB{}
	s := &Subscription{Price: 20}
	fmt.Println("top-up:", TopUp(db, 20), "balance", db.Wallet.Balance, "paid", s.Paid, "outbox", db.Outbox)
	Relay(db, s)
	fmt.Println("after the relay: paid", s.Paid, "outbox", len(db.Outbox))
	busy := &Subscription{Price: 20, Blocked: true}
	fmt.Println("one transaction:", TopUpAndRenew(&DB{}, busy, 20))
	lost := &Subscription{Price: 20}
	d2 := &DB{}
	TopUpThenSend(d2, lost, 20, true)
	fmt.Println("sent outside, crash: balance", d2.Wallet.Balance, "paid", lost.Paid)
}
