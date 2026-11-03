// ACID versus BASE: Kim sends Lee 50.
// ACID: one database, one transaction, a report never sees half of it, and a
// crash before commit rolls it back. BASE: a wallet service and a ledger
// service, an outbox event in between, a report that sees the gap, and the
// fix that counts the in-between amount as pending.
package main

import "fmt"

// DB is one database. Readers only see committed values.
type DB struct{ committed map[string]int }

// Tx holds writes that nobody else can see until Commit.
type Tx struct {
	db     *DB
	writes map[string]int
}

func (db *DB) Begin() *Tx { return &Tx{db: db, writes: map[string]int{}} }

func (db *DB) Read(key string) int { return db.committed[key] }

func (tx *Tx) Set(key string, v int) { tx.writes[key] = v } // not visible yet

// Commit makes every write visible at one instant.
func (tx *Tx) Commit() {
	for k, v := range tx.writes {
		tx.db.committed[k] = v
	}
}

// Crash before commit: the writes are thrown away (rollback).
func (tx *Tx) Crash() { tx.writes = nil }

// TransferACID moves 50 inside one transaction. crash stops it before commit.
// report runs in the middle, while the transaction is open.
func TransferACID(db *DB, crash bool, report func()) {
	tx := db.Begin()
	tx.Set("kim", db.Read("kim")-50) // written, not visible
	tx.Set("lee", db.Read("lee")+50)
	report() // a reader in the middle sees the old committed values
	if crash {
		tx.Crash() // rollback: 100 and 50 stay
		return
	}
	tx.Commit() // both visible at one instant
}

// Wallet owns Kim's balance and an outbox in the same database.
type Wallet struct {
	kim    int
	outbox []int // amounts of TransferSent events not yet confirmed
}

// Ledger owns Lee's balance.
type Ledger struct{ lee int }

// Debit is one local transaction: the balance and the outbox row together.
func (w *Wallet) Debit(amount int) {
	w.kim -= amount
	w.outbox = append(w.outbox, amount) // same commit as the debit
}

// Deliver is the relay plus the ledger: the event lands, Lee is credited.
func Deliver(w *Wallet, l *Ledger) {
	for _, amount := range w.outbox {
		l.lee += amount
	}
	w.outbox = nil // confirmed
}

// ReportNaive adds the two balances. In the gap it is short.
func ReportNaive(w *Wallet, l *Ledger) int { return w.kim + l.lee }

// ReportPending also counts transfers still in flight.
func ReportPending(w *Wallet, l *Ledger) (total, pending int) {
	for _, a := range w.outbox {
		pending += a
	}
	return w.kim + l.lee + pending, pending
}

func main() {
	fmt.Println("== ACID")
	db := &DB{committed: map[string]int{"kim": 100, "lee": 50}}
	TransferACID(db, false, func() {
		fmt.Printf("report during the transaction: %d + %d = %d\n", db.Read("kim"), db.Read("lee"), db.Read("kim")+db.Read("lee"))
	})
	fmt.Printf("after commit: kim %d, lee %d, total %d\n", db.Read("kim"), db.Read("lee"), db.Read("kim")+db.Read("lee"))
	db = &DB{committed: map[string]int{"kim": 100, "lee": 50}}
	TransferACID(db, true, func() {})
	fmt.Printf("crash before commit: kim %d, lee %d, total %d\n", db.Read("kim"), db.Read("lee"), db.Read("kim")+db.Read("lee"))

	fmt.Println("== BASE")
	w, l := &Wallet{kim: 100}, &Ledger{lee: 50}
	w.Debit(50)
	fmt.Printf("wallet committed: kim %d, event in the outbox\n", w.kim)
	fmt.Printf("report in the gap: %d + %d = %d: 50 short\n", w.kim, l.lee, ReportNaive(w, l))
	total, pending := ReportPending(w, l)
	fmt.Printf("fix, count pending: total %d, of which %d pending\n", total, pending)
	Deliver(w, l)
	fmt.Printf("event delivered: kim %d, lee %d, total %d\n", w.kim, l.lee, ReportNaive(w, l))
}
