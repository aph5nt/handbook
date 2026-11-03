// Locks and deadlocks: the graph sync and a migration both write nodes A and B.
// Part one locks in two orders and deadlocks. Part two sorts the keys first.
// Part three is optimistic locking with a version column, then seat 12A.
// Run: go run .
package main

import (
	"fmt"
	"sort"
	"strings"
)

// LockTable is the engine's lock manager: one exclusive lock per node,
// and a wait-for graph of which transaction waits for which node.
type LockTable struct {
	holder map[string]string // node -> transaction that holds its lock
	waits  map[string]string // transaction -> node it waits for
}

func NewLockTable() *LockTable {
	return &LockTable{holder: map[string]string{}, waits: map[string]string{}}
}

// Lock asks for an exclusive lock. If another transaction holds it,
// the caller waits: an edge goes into the wait-for graph.
func (t *LockTable) Lock(tx, node string) bool {
	if h, ok := t.holder[node]; ok && h != tx {
		t.waits[tx] = node
		return false
	}
	t.holder[node] = tx
	delete(t.waits, tx)
	return true
}

// Held counts the locks a transaction holds.
func (t *LockTable) Held(tx string) int {
	n := 0
	for _, h := range t.holder {
		if h == tx {
			n++
		}
	}
	return n
}

// Cycle follows the wait-for graph from tx: tx waits for a node,
// the node's holder waits for another node, and so on.
// It returns the path when the walk comes back to tx: a deadlock.
func (t *LockTable) Cycle(tx string) []string {
	path := []string{tx}
	cur := tx
	for len(path) < 20 {
		node, ok := t.waits[cur]
		if !ok {
			return nil // somebody in the chain is running: no cycle
		}
		h := t.holder[node]
		path = append(path, node, h)
		if h == tx {
			return path
		}
		cur = h
	}
	return nil
}

// Release is commit or abort: every lock of tx is freed.
func (t *LockTable) Release(tx string) {
	for node, h := range t.holder {
		if h == tx {
			delete(t.holder, node)
		}
	}
	delete(t.waits, tx)
}

func twoOrders() {
	fmt.Println("== two writers, two lock orders")
	t := NewLockTable()
	sync := []string{"A", "B"}      // page order
	migration := []string{"B", "A"} // key order
	fmt.Printf("sync order %v, migration order %v\n", sync, migration)
	fmt.Printf("sync locks A: %v\n", t.Lock("sync", sync[0]))
	fmt.Printf("migration locks B: %v\n", t.Lock("migration", migration[0]))
	fmt.Printf("sync locks B: %v, sync waits\n", t.Lock("sync", sync[1]))
	fmt.Printf("cycle from sync: %d steps, none (the migration still runs)\n", len(t.Cycle("sync")))
	fmt.Printf("migration locks A: %v, migration waits\n", t.Lock("migration", migration[1]))
	cycle := t.Cycle("migration")
	fmt.Printf("cycle from migration: %s: a deadlock\n", strings.Join(cycle, " -> "))
	t.Release("migration") // the engine picks a victim and aborts it
	fmt.Println("engine aborts migration: its lock on B is freed, its work is thrown away")
	fmt.Printf("sync locks B: %v, sync commits\n", t.Lock("sync", "B"))
	t.Release("sync")
	ok := t.Lock("migration", "B") && t.Lock("migration", "A")
	t.Release("migration")
	fmt.Printf("migration retries alone: %v; the same two orders can deadlock again\n", ok)
}

func oneOrder() {
	fmt.Println("== fix: both writers sort their keys")
	t := NewLockTable()
	sync := []string{"A", "B"}
	migration := []string{"B", "A"}
	sort.Strings(sync)
	sort.Strings(migration) // now A, B as well
	fmt.Printf("sync order %v, migration order %v\n", sync, migration)
	fmt.Printf("sync locks A: %v\n", t.Lock("sync", sync[0]))
	waited := !t.Lock("migration", migration[0])
	held := t.Held("migration")
	fmt.Printf("migration waits for A: %v, locks it holds while waiting: %d\n", waited, held)
	fmt.Printf("cycle from migration: %d steps, none\n", len(t.Cycle("migration")))
	fmt.Printf("sync locks B: %v, sync commits\n", t.Lock("sync", sync[1]))
	t.Release("sync")
	ok := t.Lock("migration", migration[0]) && t.Lock("migration", migration[1])
	t.Release("migration")
	fmt.Printf("migration locks A then B: %v, commits; aborted: 0\n", ok)
}

// Row is one row with a version column, for optimistic locking.
type Row struct {
	version int
	holder  string
}

// UpdateIfVersion is UPDATE ... SET version = version + 1 WHERE id = :id AND version = :seen.
// It returns the number of rows changed.
func (r *Row) UpdateIfVersion(seen int, who string) (rows int) {
	if r.version != seen {
		return 0
	}
	r.version++
	r.holder = who
	return 1
}

func optimistic() {
	fmt.Println("== optimistic: profile 7 with a version column")
	p := &Row{version: 3}
	ana, ben := p.version, p.version
	fmt.Printf("Ana reads version %d, Ben reads version %d, no lock taken\n", ana, ben)
	rows := p.UpdateIfVersion(ana, "Ana")
	fmt.Printf("Ana writes WHERE version = %d: %d row, now version %d\n", ana, rows, p.version)
	rows = p.UpdateIfVersion(ben, "Ben")
	fmt.Printf("Ben writes WHERE version = %d: %d rows, someone else won\n", ben, rows)
	ben = p.version // re-read, apply the edit again
	rows = p.UpdateIfVersion(ben, "Ben")
	fmt.Printf("Ben re-reads %d and writes again: %d row, now version %d\n", ben, rows, p.version)
}

func seat() {
	fmt.Println("== seat 12A, six buyers")
	s := &Row{version: 1}
	holds, wasted := 0, 0
	for b := 1; b <= 6; b++ {
		seen := 1 // all six read the free seat at version 1
		holds++   // each takes a payment hold before the write
		if s.UpdateIfVersion(seen, fmt.Sprintf("buyer %d", b)) == 0 {
			wasted++ // 0 rows: undo the hold, re-read, retry
		}
	}
	fmt.Printf("optimistic: winner %s, payment holds %d, writes with 0 rows: %d wasted\n", s.holder, holds, wasted)

	t := NewLockTable()
	waiting := 0
	for b := 1; b <= 6; b++ {
		if !t.Lock(fmt.Sprintf("buyer %d", b), "12A") { // SELECT ... FOR UPDATE
			waiting++ // waits in line, holds nothing, does no work yet
		}
	}
	fmt.Printf("FOR UPDATE: buyer 1 holds the lock, %d wait in line\n", waiting)
	t.Release("buyer 1") // buyer 1 pays once and commits
	fmt.Println("FOR UPDATE: buyer 1 buys, the others read sold: payment holds 1, wasted 0")
}

func main() {
	twoOrders()
	oneOrder()
	optimistic()
	seat()
}
