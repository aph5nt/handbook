// Lock escalation: intent locks on the swaps table, then a bulk update
// that crosses the escalation threshold and stalls every other writer.
// Run: go run .
package main

import "fmt"

const tableRows = 1_000_000 // rows in the swaps table
const escalateAt = 5_000    // SQL Server swaps row locks for a table lock near here

// Table is the lock state of the swaps table: intent flags on the table,
// an optional exclusive table lock, and exclusive locks on single rows.
type Table struct {
	intent   map[string]bool // writers holding an intent-exclusive (IX) lock
	owner    string          // writer holding the exclusive table lock, if any
	rows     map[int]string  // row -> writer holding its exclusive lock
	held     map[string]int  // writer -> row locks held
	escalate bool            // SQL Server style escalation on or off
}

func NewTable(escalate bool) *Table {
	return &Table{intent: map[string]bool{}, rows: map[int]string{}, held: map[string]int{}, escalate: escalate}
}

// LockRow puts IX on the table, then X on the row.
// It returns false when another writer owns the whole table: the caller waits.
func (t *Table) LockRow(w string, row int) bool {
	if t.owner != "" && t.owner != w {
		return false // the table lock conflicts with IX
	}
	if t.owner == w {
		return true // the table lock already covers the row
	}
	t.intent[w] = true // the flag: some row below is locked
	t.rows[row] = w
	t.held[w]++
	if t.escalate && t.held[w] >= escalateAt {
		t.Escalate(w)
	}
	return true
}

// Escalate swaps every row lock of w for one exclusive table lock.
func (t *Table) Escalate(w string) {
	for row, h := range t.rows {
		if h == w {
			delete(t.rows, row)
		}
	}
	t.held[w] = 0
	delete(t.intent, w)
	t.owner = w
}

// LockTable asks for an exclusive table lock. It reads the intent flags once:
// any IX held by another writer conflicts, so the caller waits.
func (t *Table) LockTable(w string) (ok bool, checks int) {
	checks = 1
	for other := range t.intent {
		if other != w {
			return false, checks
		}
	}
	t.owner = w
	return true, checks
}

// LockTableByScan is the same question without intent flags:
// it must look at the lock state of every row.
func (t *Table) LockTableByScan(w string) (ok bool, checks int) {
	ok = true
	for row := 0; row < tableRows; row++ {
		checks++
		if h, locked := t.rows[row]; locked && h != w {
			ok = false
		}
	}
	return ok, checks
}

// Commit frees every lock of w.
func (t *Table) Commit(w string) {
	for row, h := range t.rows {
		if h == w {
			delete(t.rows, row)
		}
	}
	t.held[w] = 0
	delete(t.intent, w)
	if t.owner == w {
		t.owner = ""
	}
}

func intentLocks() {
	fmt.Println("== intent locks: a nightly job wants the whole table")
	t := NewTable(false)
	t.LockRow("writer 1", 7)
	t.LockRow("writer 2", 9)
	fmt.Printf("writer 1 locks row 7, writer 2 locks row 9: IX flags on the table %d\n", len(t.intent))
	_, scanChecks := t.LockTableByScan("job")
	fmt.Printf("without the flag: the job checks %d rows\n", scanChecks)
	ok, checks := t.LockTable("job")
	fmt.Printf("with the flag: %d check, table lock granted %v, the job waits\n", checks, ok)
	t.Commit("writer 1")
	t.Commit("writer 2")
	ok, _ = t.LockTable("job")
	fmt.Printf("writers commit, IX flags %d, table lock granted %v\n", len(t.intent), ok)
}

func bulkUpdate() {
	fmt.Println("== bulk update: every swap of pool 42, rows 0 to 5,999")
	t := NewTable(true)
	escalatedAt := 0
	for row := 0; row < 6_000; row++ {
		before := t.held["bulk"]
		t.LockRow("bulk", row)
		if escalatedAt == 0 && t.owner == "bulk" {
			escalatedAt = before + 1
		}
	}
	fmt.Printf("escalated at %d row locks: row locks now %d, table owner %s\n", escalatedAt, len(t.rows), t.owner)
	ok := t.LockRow("app", 900_000) // a swap of pool 77, a row the bulk update never touches
	fmt.Printf("app inserts row 900000 for pool 77: granted %v, it waits for the bulk commit\n", ok)
	t.Commit("bulk")
	ok = t.LockRow("app", 900_000)
	fmt.Printf("after the bulk commit: granted %v\n", ok)
}

func batched() {
	fmt.Println("== fix: batches of 1,000 rows, each its own transaction")
	t := NewTable(true)
	most, escalations, appOK := 0, 0, true
	for start := 0; start < 6_000; start += 1_000 {
		for row := start; row < start+1_000; row++ {
			t.LockRow("bulk", row)
		}
		most = max(most, t.held["bulk"])
		if t.owner == "bulk" {
			escalations++
		}
		appOK = appOK && t.LockRow("app", 900_000+start) // the app runs between and during batches
		t.Commit("bulk")
		t.Commit("app")
	}
	fmt.Printf("batches 6, most row locks held at once %d, escalations %d, app never waits %v\n", most, escalations, appOK)
}

func postgres() {
	fmt.Println("== PostgreSQL: the row lock lives in the row")
	lockedBy := make([]string, 6_000) // each row's own header holds its locker
	for row := range lockedBy {
		lockedBy[row] = "bulk"
	}
	lockTableEntries := 0 // nothing is added to a shared lock table per row
	fmt.Printf("rows locked %d, lock table entries %d, escalations 0\n", len(lockedBy), lockTableEntries)
}

func main() {
	intentLocks()
	bulkUpdate()
	batched()
	postgres()
}
