// Command expandcontract replays the lesson: the column dest becomes to_address. A table
// is a set of columns plus rows. Done in six releases, no version ever sees a missing
// column. Done in one step, the old version fails and rollback fails too.
package main

import "fmt"

// Table is a set of columns and its rows, column name -> value.
type Table struct {
	Columns map[string]bool
	Rows    []map[string]string
}

// Write inserts one row into the named columns. It fails if a column is missing.
func (t *Table) Write(addr string, cols ...string) error {
	row := map[string]string{}
	for _, c := range cols {
		if !t.Columns[c] {
			return fmt.Errorf("insert failed: column %s does not exist", c)
		}
		row[c] = addr
	}
	t.Rows = append(t.Rows, row)
	return nil
}

// Read counts the rows with a value in the column, or fails if the column is missing.
func (t *Table) Read(col string) (int, error) {
	if !t.Columns[col] {
		return 0, fmt.Errorf("query failed: column %s does not exist", col)
	}
	n := 0
	for _, r := range t.Rows {
		if r[col] != "" {
			n++
		}
	}
	return n, nil
}

// Backfill copies from into to on every row that has no value in to, a range at a time.
func (t *Table) Backfill(from, to string, rangeSize int) (copied, ranges int) {
	for start := 0; start < len(t.Rows); start += rangeSize {
		end := min(start+rangeSize, len(t.Rows))
		for _, r := range t.Rows[start:end] {
			if r[to] == "" {
				r[to] = r[from]
				copied++
			}
		}
		ranges++
	}
	return
}

func report(step string, t *Table) {
	d, derr := t.Read("dest")
	a, aerr := t.Read("to_address")
	switch {
	case derr != nil:
		fmt.Printf("%-34s to_address %d rows, dest dropped\n", step, a)
	case aerr != nil:
		fmt.Printf("%-34s dest %d rows, to_address missing\n", step, d)
	default:
		fmt.Printf("%-34s dest %d rows, to_address %d rows\n", step, d, a)
	}
}

func fresh() *Table {
	t := &Table{Columns: map[string]bool{"dest": true}}
	for k := 0; k < 1000; k++ {
		_ = t.Write("0xabc", "dest") // v1 writes dest
	}
	return t
}

func main() {
	t := fresh()
	report("start:", t)

	t.Columns["to_address"] = true // 1. expand: add the column, empty
	report("1. expand:", t)

	for k := 0; k < 100; k++ { // 2. the roll: v2 writes both, v1 still writes dest
		if k%2 == 0 {
			_ = t.Write("0xabc", "dest", "to_address")
		} else {
			_ = t.Write("0xabc", "dest")
		}
	}
	report("2. dual write, roll half done:", t)

	copied, ranges := t.Backfill("dest", "to_address", 500) // 3. backfill old rows
	fmt.Printf("3. backfill: %d rows copied in %d ranges\n", copied, ranges)
	report("   after backfill:", t)

	n, err := t.Read("to_address") // 4. readers switch
	fmt.Printf("4. switch reads: %d rows, error %v\n", n, err)

	_ = t.Write("0xabc", "to_address") // 5. v3 writes only to_address
	report("5. stop writing dest:", t)

	delete(t.Columns, "dest") // 6. contract
	report("6. contract:", t)

	// The rename in one step. The roll takes 10 minutes, 4 copies, 25 inserts a minute each.
	fmt.Println()
	t = fresh()
	delete(t.Columns, "dest")
	t.Columns["to_address"] = true
	failed := 0
	for m := 0; m < 10; m++ {
		v1Copies := 4 - m*2/5 // one copy replaced every 2.5 minutes
		for k := 0; k < v1Copies*25; k++ {
			if err := t.Write("0xabc", "dest"); err != nil {
				failed++
			}
		}
	}
	fmt.Printf("one-step rename: %d inserts failed during the 10 minute roll\n", failed)
	_, err = t.Read("dest")
	fmt.Println("reader released last week:", err)
	err = t.Write("0xabc", "dest")
	fmt.Println("roll back to v1:", err)
}
