// Upserts and deduplication: swap 9507 log 1 is written three times,
// version 1 (bug, 125), version 2 (fixed, 12.5), then a late replay of
// version 1. A keyed table with a version keeps one correct row.
package main

import "fmt"

// Key names the event: block height plus log index.
type Key struct {
	Block, Log int
}

type Row struct {
	Key     Key
	Version int
	Amount  float64
}

// Table is a primary-key table. useVersion switches the version check.
type Table struct {
	rows       map[Key]Row
	useVersion bool
}

// Upsert updates the row if the key exists, inserts it if not. With a
// version, an older write never beats a newer one.
func (t *Table) Upsert(r Row) string {
	old, ok := t.rows[r.Key]
	if !ok {
		t.rows[r.Key] = r
		return "insert"
	}
	if t.useVersion && r.Version < old.Version {
		return fmt.Sprintf("ignored: version %d < %d", r.Version, old.Version) // version 1 < 2
	}
	t.rows[r.Key] = r
	return "update"
}

// DedupAtRead keeps one row per key, the highest version, like
// ROW_NUMBER() OVER (PARTITION BY key ORDER BY version DESC) = 1.
func DedupAtRead(rows []Row) map[Key]Row {
	best := map[Key]Row{}
	for _, r := range rows {
		if b, ok := best[r.Key]; !ok || r.Version > b.Version {
			best[r.Key] = r
		}
	}
	return best
}

const appendSQL = `-- append only: every write adds a row
INSERT INTO swaps_append (block_height, log_index, version, amount)
VALUES (9507, 1, 1, 125), (9507, 1, 2, 12.5), (9507, 1, 1, 125);

-- 262.5, the truth is 12.5
SELECT sum(amount) FROM swaps_append;

-- the fix at write time: a primary-key table keyed by the event
CREATE TABLE swaps (block_height BIGINT, log_index INT,
                    version INT, amount DECIMAL(38, 9))
PRIMARY KEY (block_height, log_index);

-- or deduplicate at read: one row per key, highest version
SELECT * FROM (
  SELECT *, ROW_NUMBER() OVER (PARTITION BY block_height, log_index
                               ORDER BY version DESC) AS rn
  FROM swaps_append) t
WHERE rn = 1;`

func main() {
	k := Key{9507, 1}
	writes := []Row{{k, 1, 125}, {k, 2, 12.5}, {k, 1, 125}}

	pk := &Table{rows: map[Key]Row{}, useVersion: true}
	for i, w := range writes {
		fmt.Printf("primary key + version, write %d (v%d %.1f): %s\n", i+1, w.Version, w.Amount, pk.Upsert(w))
	}
	fmt.Printf("  result: %d row, amount %.1f\n", len(pk.rows), pk.rows[k].Amount)

	lww := &Table{rows: map[Key]Row{}, useVersion: false}
	for _, w := range writes {
		lww.Upsert(w)
	}
	fmt.Printf("primary key, no version (last write wins): amount %.1f\n", lww.rows[k].Amount)

	var appendOnly []Row
	sum := 0.0
	for _, w := range writes {
		appendOnly = append(appendOnly, w)
		sum += w.Amount
	}
	fmt.Printf("append only: %d rows, SUM(amount) %.1f, truth 12.5\n", len(appendOnly), sum)
	d := DedupAtRead(appendOnly)
	fmt.Printf("dedup at read: %d row, amount %.1f\n", len(d), d[k].Amount)
	_ = appendSQL
}
