// Lost update: two agent tool calls each spend 8 from wallet 7, which holds 10.
// The first part loses one spend. The next three parts are the three fixes.
// Run: go run .
package main

import (
	"fmt"
	"sync"
)

const cost = 8 // what each tool call spends

// Wallet is one row of the wallet table: id 7, a balance and a version column.
type Wallet struct {
	mu      sync.Mutex // the row lock that SELECT ... FOR UPDATE takes
	balance int
	version int
}

// Read is a plain SELECT balance, version: no lock, no check.
func (w *Wallet) Read() (balance, version int) {
	return w.balance, w.version
}

// Write is UPDATE wallet SET balance = :new. It overwrites whatever is there.
func (w *Wallet) Write(newBalance int) {
	w.balance = newBalance
	w.version++
}

// Spend is UPDATE wallet SET balance = balance - :cost WHERE id = 7 AND balance >= :cost.
// The check and the write are one step. It returns the number of rows changed.
func (w *Wallet) Spend(c int) (rows int) {
	if w.balance < c {
		return 0 // the WHERE is false: nothing changes
	}
	w.balance -= c
	w.version++
	return 1
}

// WriteIfVersion is UPDATE wallet SET balance = :new, version = version + 1
// WHERE id = 7 AND version = :seen. Zero rows means someone wrote first.
func (w *Wallet) WriteIfVersion(newBalance, seen int) (rows int) {
	if w.version != seen {
		return 0
	}
	w.balance = newBalance
	w.version++
	return 1
}

func lost() {
	fmt.Println("== lost update: read, compute in the app, write back")
	w := &Wallet{balance: 10, version: 1}
	a, _ := w.Read()
	fmt.Printf("call A reads %d, plans %d\n", a, a-cost)
	b, _ := w.Read()
	fmt.Printf("call B reads %d, plans %d\n", b, b-cost)
	w.Write(a - cost)
	fmt.Printf("call A writes %d\n", w.balance)
	w.Write(b - cost) // B's write lands on top of A's
	fmt.Printf("call B writes %d: %d spent from 10, A's spend is lost\n", w.balance, 2*cost)
}

func fixOneStatement() {
	fmt.Println("== fix one: check and write in one statement")
	w := &Wallet{balance: 10, version: 1}
	rows := w.Spend(cost)
	fmt.Printf("call A: %d row changed, balance %d\n", rows, w.balance)
	rows = w.Spend(cost)
	fmt.Printf("call B: %d rows changed, balance %d: not enough money\n", rows, w.balance)
}

func fixLockFirst() {
	fmt.Println("== fix two: lock the row first")
	w := &Wallet{balance: 10, version: 1}
	w.mu.Lock() // call A: SELECT ... FOR UPDATE takes the row lock
	a, _ := w.Read()
	fmt.Printf("call A locks the row and reads %d\n", a)
	got := w.mu.TryLock() // call B: the same SELECT ... FOR UPDATE
	fmt.Printf("call B asks for the lock: got it = %v, so B waits\n", got)
	w.Write(a - cost)
	w.mu.Unlock() // commit releases the lock
	fmt.Printf("call A writes %d and commits\n", w.balance)
	w.mu.Lock() // B's wait ends
	b, _ := w.Read()
	if b < cost {
		fmt.Printf("call B reads %d: cannot pay %d, refuses\n", b, cost)
	}
	w.mu.Unlock()
}

func fixVersion() {
	fmt.Println("== fix three: a version column")
	w := &Wallet{balance: 10, version: 1}
	a, va := w.Read()
	b, vb := w.Read()
	fmt.Printf("both read balance %d at version %d\n", a, va)
	rows := w.WriteIfVersion(a-cost, va)
	fmt.Printf("call A writes WHERE version = %d: %d row, now version %d\n", va, rows, w.version)
	rows = w.WriteIfVersion(b-cost, vb)
	fmt.Printf("call B writes WHERE version = %d: %d rows\n", vb, rows)
	b, _ = w.Read() // re-read after zero rows
	if b < cost {
		fmt.Printf("call B re-reads %d: cannot pay %d, refuses\n", b, cost)
	}
}

func main() {
	lost()
	fixOneStatement()
	fixLockFirst()
	fixVersion()
}
