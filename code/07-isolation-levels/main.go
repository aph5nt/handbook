// Isolation levels: one table of versioned rows, and one read rule per level.
// Each scene replays two transactions, shows the anomaly the level lets
// through, then replays at the level that stops it.
// Run: go run .
package main

import (
	"errors"
	"fmt"
)

// Level is the isolation level a transaction runs at.
type Level int

const (
	ReadUncommitted Level = iota
	ReadCommitted
	RepeatableRead // ANSI: rows already read stay the same
	Snapshot       // one frozen picture of the whole table
	Serializable   // snapshot plus a check at commit
)

// Version is one value of a row, written by one transaction.
type Version struct {
	value     int
	writer    int
	committed int // commit sequence number; 0 = not committed yet
}

// Row is one row: a name, a group (the pool, or the on-call list), its versions.
type Row struct {
	name     string
	group    string
	versions []*Version
}

// DB holds the rows, a commit counter, and the transactions that committed.
type DB struct {
	rows   []*Row
	clock  int
	nextTx int
	done   []*Tx
}

// Tx is one transaction.
type Tx struct {
	db       *DB
	id       int
	level    Level
	snapshot int          // set at the first statement; -1 before it
	frozen   map[*Row]int // ANSI repeatable read: values already read
	reads    map[*Row]bool
	writes   map[*Row]bool
	commit   int
}

func (db *DB) Add(name, group string, value int) *Row {
	db.clock++
	r := &Row{name: name, group: group, versions: []*Version{{value: value, committed: db.clock}}}
	db.rows = append(db.rows, r)
	return r
}

func (db *DB) Begin(level Level) *Tx {
	db.nextTx++
	return &Tx{db: db, id: db.nextTx, level: level, snapshot: -1, frozen: map[*Row]int{}, reads: map[*Row]bool{}, writes: map[*Row]bool{}}
}

// visible is the read rule of each level: which version this tx may see.
func (t *Tx) visible(r *Row) *Version {
	if t.snapshot < 0 {
		t.snapshot = t.db.clock // the picture is taken at the first statement
	}
	for j := len(r.versions) - 1; j >= 0; j-- {
		v := r.versions[j]
		switch {
		case v.writer == t.id && v.committed == 0:
			return v // a transaction sees its own writes
		case t.level == ReadUncommitted:
			return v // the newest version, committed or not
		case t.level <= RepeatableRead && v.committed > 0:
			return v // the newest committed version, now
		case v.committed > 0 && v.committed <= t.snapshot:
			return v // committed before my snapshot
		}
	}
	return nil // the row did not exist for me
}

// Read reads one row. Under ANSI repeatable read a row read once is frozen.
func (t *Tx) Read(r *Row) int {
	if val, ok := t.frozen[r]; ok && t.level == RepeatableRead {
		return val
	}
	v := t.visible(r)
	if v == nil {
		return 0
	}
	t.reads[r] = true
	if t.level == RepeatableRead {
		t.frozen[r] = v.value
	}
	return v.value
}

// List is a range query: every row of the group with a value above 0.
// A row that arrived after the first read is not frozen, so ANSI repeatable
// read sees it: a phantom.
func (t *Tx) List(group string) []string {
	var names []string
	for _, r := range t.db.rows {
		if r.group == group && t.Read(r) > 0 {
			names = append(names, r.name)
		}
	}
	return names
}

// Write adds a new, uncommitted version. Other levels cannot see it yet.
func (t *Tx) Write(r *Row, value int) {
	r.versions = append(r.versions, &Version{value: value, writer: t.id})
	t.writes[r] = true
}

// Insert adds a new row with one uncommitted version.
func (t *Tx) Insert(name, group string, value int) {
	r := &Row{name: name, group: group}
	t.db.rows = append(t.db.rows, r)
	t.Write(r, value)
}

// Rollback removes every version this tx wrote. The value never existed.
func (t *Tx) Rollback() {
	for r := range t.writes {
		kept := r.versions[:0]
		for _, v := range r.versions {
			if v.writer != t.id {
				kept = append(kept, v)
			}
		}
		r.versions = kept
	}
}

var errSerialization = errors.New("serialization failure (SQLSTATE 40001)")

// rwDep: a read a row that b wrote.
func rwDep(a, b *Tx) bool {
	for r := range a.reads {
		if b.writes[r] {
			return true
		}
	}
	return false
}

// Commit makes the writes visible. Serializable first checks for a
// transaction that committed meanwhile with read-write dependencies both
// ways: no one-at-a-time order allows that, so this one is aborted.
func (t *Tx) Commit() error {
	if t.level == Serializable {
		for _, c := range t.db.done {
			if c.commit > t.snapshot && rwDep(t, c) && rwDep(c, t) {
				t.Rollback()
				return errSerialization
			}
		}
	}
	t.db.clock++
	t.commit = t.db.clock
	for r := range t.writes {
		for _, v := range r.versions {
			if v.writer == t.id {
				v.committed = t.commit
			}
		}
	}
	t.db.done = append(t.db.done, t)
	return nil
}

func dirtyRead(level Level) (seen, after int) {
	db := &DB{}
	w := db.Add("wallet 7", "wallets", 100)
	t1 := db.Begin(ReadCommitted) // the payment
	t2 := db.Begin(level)         // the fraud check
	t1.Write(w, 20)               // not committed
	seen = t2.Read(w)             // the fraud check reads
	t1.Rollback()                 // the payment fails
	after = db.Begin(ReadCommitted).Read(w)
	return seen, after
}

func nonRepeatable(level Level) (first, second int) {
	db := &DB{}
	w := db.Add("wallet 7", "wallets", 100)
	t2 := db.Begin(level) // the report
	first = t2.Read(w)
	t1 := db.Begin(ReadCommitted) // a payment in between
	t1.Write(w, 20)
	t1.Commit()
	second = t2.Read(w)
	return first, second
}

func phantom(level Level) (count int, list []string) {
	db := &DB{}
	db.Add("swap 1", "p2", 1)
	db.Add("swap 2", "p2", 1)
	t2 := db.Begin(level) // the report
	count = len(t2.List("p2"))
	t1 := db.Begin(ReadCommitted) // the indexer
	t1.Insert("swap 3", "p2", 1)
	t1.Commit()
	return count, t2.List("p2")
}

// goOff is one request: count who is on call, go off only if 2 are.
func goOff(t *Tx, me *Row) (counted int, wentOff bool) {
	counted = len(t.List("on_call"))
	if counted < 2 {
		return counted, false
	}
	t.Write(me, 0)
	return counted, true
}

func writeSkew(level Level) {
	db := &DB{}
	ana := db.Add("Ana", "on_call", 1)
	ben := db.Add("Ben", "on_call", 1)
	t1 := db.Begin(level) // Ana's request
	t2 := db.Begin(level) // Ben's request
	n1, _ := goOff(t1, ana)
	n2, _ := goOff(t2, ben)
	fmt.Printf("  T1 counts %d, T2 counts %d; T1 sets Ana off, T2 sets Ben off\n", n1, n2)
	err1 := t1.Commit()
	err2 := t2.Commit()
	fmt.Printf("  T1 commit: %v, T2 commit: %v\n", err1, err2)
	if errors.Is(err2, errSerialization) {
		t2 = db.Begin(level) // the app retries T2 from the start
		n, off := goOff(t2, ben)
		t2.Commit()
		fmt.Printf("  T2 retries: counts %d, goes off %v\n", n, off)
	}
	check := db.Begin(ReadCommitted)
	fmt.Printf("  on call now: %v\n", check.List("on_call"))
}

func main() {
	fmt.Println("== read uncommitted: wallet 7 = 100, T1 writes 20, does not commit")
	seen, after := dirtyRead(ReadUncommitted)
	fmt.Printf("T2 at read uncommitted reads %d: a dirty read; T1 rolls back, wallet 7 = %d\n", seen, after)
	seen, _ = dirtyRead(ReadCommitted)
	fmt.Printf("fix, T2 at read committed reads %d while T1 is open\n", seen)

	fmt.Println("\n== read committed: T2 reads wallet 7 twice, T1 commits 20 in between")
	a, b := nonRepeatable(ReadCommitted)
	fmt.Printf("T2 at read committed reads %d, then %d: a non-repeatable read\n", a, b)
	a, b = nonRepeatable(RepeatableRead)
	fmt.Printf("fix, T2 at repeatable read reads %d, then %d\n", a, b)

	fmt.Println("\n== repeatable read: T2 counts the swaps of p2, T1 inserts swap 3")
	n, list := phantom(RepeatableRead)
	fmt.Printf("T2 at repeatable read counts %d, then lists %d %v: a phantom\n", n, len(list), list)
	n, list = phantom(Snapshot)
	fmt.Printf("fix, T2 at snapshot counts %d, then lists %d %v\n", n, len(list), list)

	fmt.Println("\n== snapshot: Ana and Ben on call, rule: at least one stays")
	fmt.Println("at snapshot:")
	writeSkew(Snapshot)
	fmt.Println("fix, at serializable:")
	writeSkew(Serializable)
}
