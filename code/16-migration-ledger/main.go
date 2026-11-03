// A migration runner with a ledger: numbered files, applied in order to
// every database of the family, recorded with a checksum, and refused if
// an applied file changes. Services gate on a minimum id.
package main

import (
	"crypto/sha256"
	"fmt"
	"sort"
)

type Migration struct{ ID, SQL string }

type Runner struct {
	ledger map[string]string // id -> checksum of the applied text
	dbs    map[string]string // database -> highest applied id
	order  []string          // the family, applied in this order
}

func checksum(sql string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(sql)))[:4] }

// Run checks every applied file first, then applies the new ones in order.
func (r *Runner) Run(files []Migration) error {
	sort.Slice(files, func(i, j int) bool { return files[i].ID < files[j].ID })
	for _, m := range files {
		if sum, done := r.ledger[m.ID]; done && sum != checksum(m.SQL) {
			return fmt.Errorf("checksum mismatch on %s: ledger %s, file %s: refuse", m.ID, sum, checksum(m.SQL))
		}
	}
	for _, m := range files {
		if _, done := r.ledger[m.ID]; done {
			continue // applied before: never run again
		}
		for _, db := range r.order { // one runner, the whole family
			r.dbs[db] = m.ID // stands in for executing m.SQL on db
		}
		r.ledger[m.ID] = checksum(m.SQL) // record after it ran everywhere
	}
	return nil
}

// ReadyMinimum is the readiness latch: at least this id applied.
func ReadyMinimum(applied, min string) bool { return applied >= min }

// ReadyExact is the trap: exactly this id.
func ReadyExact(applied, want string) bool { return applied == want }

func main() {
	files := []Migration{
		{"041", "ALTER TABLE swaps ADD COLUMN fee DECIMAL(38,18);"},
		{"042", "CREATE INDEX swaps_pool ON swaps (pool);"},
	}
	r := &Runner{ledger: map[string]string{}, dbs: map[string]string{}, order: []string{"ethereum", "base", "robinhood"}}
	_ = r.Run(files)
	fmt.Println("ledger:", "041", r.ledger["041"], "042", r.ledger["042"])

	files = append(files, Migration{"043", "ALTER TABLE swaps ADD COLUMN router VARCHAR(42);"})
	_ = r.Run(files)
	fmt.Println("after 043:", r.dbs, "ledger 043", r.ledger["043"])
	fmt.Println("service needs at least 042, found", r.dbs["robinhood"], "starts:", ReadyMinimum(r.dbs["robinhood"], "042"))
	fmt.Println("service needs exactly 042, found", r.dbs["robinhood"], "starts:", ReadyExact(r.dbs["robinhood"], "042"))

	edited := append([]Migration(nil), files...)
	edited[0].SQL = "ALTER TABLE swaps ADD COLUMN fee DECIMAL(38,9);" // someone edits 041
	fmt.Println("041 on disk now has checksum", checksum(edited[0].SQL))
	fmt.Println("runner:", r.Run(edited))

	// the right way: leave 041 alone, expand with 044, contract with 045
	files = append(files,
		Migration{"044", "ALTER TABLE swaps ADD COLUMN fee_v2 DECIMAL(38,9);"},
		Migration{"045", "ALTER TABLE swaps DROP COLUMN fee;"})
	fmt.Println("runner:", r.Run(files), "after 045:", r.dbs["robinhood"], "ledger 044", r.ledger["044"], "045", r.ledger["045"])
}
