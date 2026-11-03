// SSI versus 2PL: Ana and Ben are on call, and each asks to go off.
// The rule: go off only if 2 are on call. Two-phase locking (2PL) keeps the
// rule with locks and a deadlock. Serializable snapshot isolation (SSI) keeps
// it with an abort at commit, and a retry loop makes that abort harmless.
// Run: go run .
package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var people = []string{"Ana", "Ben"}

// countOnCall is the read each request makes: SELECT count(*) WHERE on_call.
func countOnCall(rows map[string]bool) int {
	n := 0
	for _, p := range people {
		if rows[p] {
			n++
		}
	}
	return n
}

// Lock is the lock entry of one row.
type Lock struct {
	shared    map[string]bool // readers holding it
	exclusive string          // the one writer holding it
}

// LockManager grants locks and records who waits for whom.
type LockManager struct {
	rows     map[string]*Lock
	waitsFor map[string]string
}

func newLockManager() *LockManager {
	m := &LockManager{rows: map[string]*Lock{}, waitsFor: map[string]string{}}
	for _, p := range people {
		m.rows[p] = &Lock{shared: map[string]bool{}}
	}
	return m
}

// Shared is the lock a read takes. Readers never block readers.
func (m *LockManager) Shared(tx, row string) bool {
	l := m.rows[row]
	if l.exclusive != "" && l.exclusive != tx {
		return false
	}
	l.shared[tx] = true
	return true
}

// Exclusive is the lock a write takes. Any other reader of the row blocks it.
func (m *LockManager) Exclusive(tx, row string) (granted bool, waitsFor string) {
	l := m.rows[row]
	for holder := range l.shared {
		if holder != tx {
			m.waitsFor[tx] = holder
			return false, holder
		}
	}
	l.exclusive = tx
	delete(m.waitsFor, tx)
	return true, ""
}

// Deadlock follows the waits-for edges from tx. Coming back to tx is a cycle.
func (m *LockManager) Deadlock(tx string) bool {
	next, ok := m.waitsFor[tx]
	for hops := 0; ok && hops <= len(m.waitsFor); hops++ {
		if next == tx {
			return true
		}
		next, ok = m.waitsFor[next]
	}
	return false
}

// Release drops every lock of tx. It runs at commit or abort, never before.
func (m *LockManager) Release(tx string) {
	for _, l := range m.rows {
		delete(l.shared, tx)
		if l.exclusive == tx {
			l.exclusive = ""
		}
	}
	delete(m.waitsFor, tx)
	for waiter, on := range m.waitsFor {
		if on == tx {
			delete(m.waitsFor, waiter)
		}
	}
}

func holders(l *Lock) string {
	var names []string
	for tx := range l.shared {
		names = append(names, tx)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func twoPhase() {
	fmt.Println("== 2PL: locks held to commit")
	onCall := map[string]bool{"Ana": true, "Ben": true}
	m := newLockManager()
	m.Shared("T1", "Ana") // T1 reads both rows
	m.Shared("T1", "Ben")
	fmt.Printf("T1 counts %d, holds shared on Ana: %s\n", countOnCall(onCall), holders(m.rows["Ana"]))
	m.Shared("T2", "Ana") // T2 reads both rows: shared locks do not conflict
	m.Shared("T2", "Ben")
	fmt.Printf("T2 counts %d; shared on Ana: %s; shared on Ben: %s\n", countOnCall(onCall), holders(m.rows["Ana"]), holders(m.rows["Ben"]))

	ok, on := m.Exclusive("T1", "Ana")
	fmt.Printf("T1 asks exclusive on Ana: granted %v, waits for %s\n", ok, on)
	ok, on = m.Exclusive("T2", "Ben")
	fmt.Printf("T2 asks exclusive on Ben: granted %v, waits for %s\n", ok, on)

	if m.Deadlock("T2") {
		fmt.Println("deadlock: T1 waits for T2, T2 waits for T1; the engine aborts T2")
		m.Release("T2") // abort: T2's locks go
	}
	ok, _ = m.Exclusive("T1", "Ana")
	onCall["Ana"] = false
	m.Release("T1") // commit
	fmt.Printf("T1 gets exclusive on Ana: %v, sets Ana off, commits\n", ok)

	// The app retries T2 from the start: new locks, a new count.
	for _, p := range people {
		m.Shared("T2", p)
	}
	n := countOnCall(onCall)
	if n < 2 {
		fmt.Printf("T2 retries: counts %d, refused; Ben on call: %v\n", n, onCall["Ben"])
	}
	m.Release("T2")
}

// Tx is one SSI transaction: its snapshot, what it read, what it wrote.
type Tx struct {
	name     string
	start    int
	commit   int
	snapshot map[string]bool
	reads    map[string]bool
	writes   map[string]bool
	counted  int
}

// DB holds the committed rows and the transactions that committed.
type DB struct {
	rows      map[string]bool
	clock     int
	committed []*Tx
}

// Begin takes a snapshot: a copy of the committed rows. No locks.
func (db *DB) Begin(name string) *Tx {
	snap := map[string]bool{}
	for k, v := range db.rows {
		snap[k] = v
	}
	return &Tx{name: name, start: db.clock, snapshot: snap, reads: map[string]bool{}, writes: map[string]bool{}}
}

// Count reads every row from the snapshot. The engine notes each read.
func (t *Tx) Count() int {
	for _, p := range people {
		t.reads[p] = true
	}
	t.counted = countOnCall(t.snapshot)
	return t.counted
}

// SetOff writes one row. Nothing waits: the write is the tx's own copy.
func (t *Tx) SetOff(person string) {
	t.writes[person] = false
}

// rwDep lists rows a read that b then wrote: a read-write dependency a -> b.
func rwDep(a, b *Tx) []string {
	var rows []string
	for _, p := range people {
		_, wrote := b.writes[p]
		if a.reads[p] && wrote {
			rows = append(rows, p)
		}
	}
	return rows
}

var errSerialization = errors.New("could not serialize access due to read/write dependencies among transactions (SQLSTATE 40001)")

// Commit checks for a dangerous pair: a transaction that committed while this
// one ran, with read-write dependencies in both directions. No serial order
// gives that result, so the later committer is aborted.
func (db *DB) Commit(t *Tx) error {
	for _, c := range db.committed {
		concurrent := c.commit > t.start
		if concurrent && len(rwDep(t, c)) > 0 && len(rwDep(c, t)) > 0 {
			return errSerialization
		}
	}
	for row, v := range t.writes {
		db.rows[row] = v
	}
	db.clock++
	t.commit = db.clock
	db.committed = append(db.committed, t)
	return nil
}

// goOff is the whole request: count, decide, write.
func goOff(t *Tx, person string) bool {
	if t.Count() < 2 {
		return false // the rule refuses
	}
	t.SetOff(person)
	return true
}

func ssi() {
	fmt.Println("== SSI: snapshots, checked at commit")
	db := &DB{rows: map[string]bool{"Ana": true, "Ben": true}}
	t1 := db.Begin("T1")
	t2 := db.Begin("T2")
	n1 := t1.Count()
	n2 := t2.Count()
	fmt.Printf("T1 counts %d, T2 counts %d, from their snapshots: no locks, no wait\n", n1, n2)

	t1.SetOff("Ana")
	t2.SetOff("Ben")
	fmt.Println("T1 writes Ana off, T2 writes Ben off: different rows, no wait")

	fmt.Printf("rw dependencies: T1 -> T2 on %v, T2 -> T1 on %v\n", rwDep(t1, t2), rwDep(t2, t1))
	err1 := db.Commit(t1)
	fmt.Printf("T1 commit: error %v; Ana on call: %v\n", err1, db.rows["Ana"])
	err2 := db.Commit(t2)
	fmt.Printf("T2 commit: ERROR: %v; Ben on call: %v\n", err2, db.rows["Ben"])
	fmt.Println("no retry loop: Ben sees the error page")

	// Fix: retry the whole transaction on SQLSTATE 40001.
	for attempt := 2; attempt <= 3; attempt++ {
		t := db.Begin("T2")
		off := goOff(t, "Ben")
		if err := db.Commit(t); errors.Is(err, errSerialization) {
			continue
		}
		fmt.Printf("retry attempt %d: fresh snapshot counts %d, goes off %v; Ben on call: %v\n", attempt, t.counted, off, db.rows["Ben"])
		break
	}
}

func main() {
	twoPhase()
	fmt.Println()
	ssi()
}
