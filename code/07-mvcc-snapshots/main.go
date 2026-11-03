// MVCC snapshots: row 7 keeps every version, each reader sees the version
// its snapshot allows, and cleanup (VACUUM) removes only what no snapshot
// can see. The second part shows bloat: one open snapshot blocks cleanup.
// Run: go run .
package main

import (
	"fmt"
	"strings"
)

// Version is one copy of a row. made is the transaction that wrote it
// (xmin in PostgreSQL), ended the one that replaced it (xmax), 0 = live.
type Version struct {
	name    string
	balance int
	made    int
	ended   int
}

// Row is one table row: a list of versions, oldest first.
type Row struct {
	versions []*Version
}

// Update never overwrites. It ends the live version and adds a new one.
func (r *Row) Update(tx, balance int) {
	live := r.versions[len(r.versions)-1]
	live.ended = tx
	name := fmt.Sprintf("v%d", len(r.versions)+1)
	r.versions = append(r.versions, &Version{name: name, balance: balance, made: tx})
}

// Snapshot is fixed when the reader starts: it sees transactions before tx.
type Snapshot struct {
	tx int
}

// Visible is the rule: made before my snapshot, and not ended before it.
func (s Snapshot) Visible(v *Version) bool {
	madeBefore := v.made < s.tx
	stillLive := v.ended == 0 || v.ended > s.tx
	return madeBefore && stillLive
}

// Read returns the one version this snapshot can see.
func (s Snapshot) Read(r *Row) *Version {
	for _, v := range r.versions {
		if s.Visible(v) {
			return v
		}
	}
	return nil
}

// Vacuum removes versions ended at or before the horizon: the oldest open
// snapshot. A version ended after it may still be read, so it stays.
func (r *Row) Vacuum(horizon int) (removed int) {
	kept := r.versions[:0]
	for _, v := range r.versions {
		if v.ended != 0 && v.ended <= horizon {
			removed++
			continue
		}
		kept = append(kept, v)
	}
	r.versions = kept
	return removed
}

func versions() {
	fmt.Println("== row 7 over time")
	row := &Row{versions: []*Version{{name: "v1", balance: 10, made: 100}}}
	fmt.Println("v1 balance 10, made by tx 100, live")

	report := Snapshot{tx: 110}
	v := report.Read(row)
	fmt.Printf("report snapshot tx %d reads %s: %d\n", report.tx, v.name, v.balance)

	row.Update(120, 2)
	v = report.Read(row)
	fmt.Printf("tx 120 makes v2 balance 2, v1 ended by 120; report still reads %s: %d\n", v.name, v.balance)

	row.Update(140, 50)
	fmt.Printf("tx 140 makes v3 balance 50; versions kept: %d\n", len(row.versions))

	app := Snapshot{tx: 150}
	a := app.Read(row)
	r := report.Read(row)
	fmt.Printf("app snapshot tx %d reads %s: %d, report reads %s: %d\n", app.tx, a.name, a.balance, r.name, r.balance)

	// The report finishes. The oldest open snapshot is now the app's, tx 150.
	removed := row.Vacuum(app.tx)
	fmt.Printf("report done, VACUUM horizon tx %d removes %d (v1, v2), keeps %d\n", app.tx, removed, len(row.versions))
}

const wallets = 36 // rows in table wallets

// Table is table wallets: 36 rows, each with its own versions.
type Table struct {
	rows []*Row
	tx   int
}

func newTable() *Table {
	t := &Table{tx: 900}
	for i := 0; i < wallets; i++ {
		t.rows = append(t.rows, &Row{versions: []*Version{{name: "v1", balance: 10, made: 899}}})
	}
	return t
}

// UpdateSome updates n wallets, one transaction each.
func (t *Table) UpdateSome(n int) {
	for i := 0; i < n; i++ {
		t.tx++
		t.rows[i%wallets].Update(t.tx, 5)
	}
}

func (t *Table) Dead() int {
	dead := 0
	for _, r := range t.rows {
		dead += len(r.versions) - 1
	}
	return dead
}

// Size is the stored versions against the live rows: 1x means no garbage.
func (t *Table) Size() string {
	stored := wallets + t.Dead()
	f := fmt.Sprintf("%.1f", float64(stored)/wallets)
	return strings.TrimSuffix(f, ".0") + "x"
}

func (t *Table) Vacuum(horizon int) (removed int) {
	for _, r := range t.rows {
		removed += r.Vacuum(horizon)
	}
	return removed
}

func bloat() {
	fmt.Println("== table wallets, one hour")
	t := newTable()
	report := Snapshot{tx: t.tx} // 09:00: the hour-long report starts
	fmt.Printf("09:00 report holds snapshot tx %d; %d wallets, size %s\n", report.tx, wallets, t.Size())

	t.UpdateSome(12)
	fmt.Printf("12 updates: dead versions %d, size %s\n", t.Dead(), t.Size())

	t.UpdateSome(12)
	removed := t.Vacuum(report.tx) // the report is the oldest open snapshot
	fmt.Printf("09:30 VACUUM horizon tx %d: removed %d, dead versions %d, size %s\n", report.tx, removed, t.Dead(), t.Size())

	t.UpdateSome(12)
	fmt.Printf("10:00 dead versions %d, size %s: bloat\n", t.Dead(), t.Size())

	// Fix: the report runs on the warehouse, so no app snapshot stays open.
	horizon := t.tx
	removed = t.Vacuum(horizon)
	fmt.Printf("fix: no open snapshot, VACUUM horizon tx %d: removed %d, size %s\n", horizon, removed, t.Size())
}

func main() {
	versions()
	fmt.Println()
	bloat()
}
