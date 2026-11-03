// Copy-on-write versus merge-on-read: fix one wrong row in an immutable
// data file, and count what writers and readers must do. Replays the lesson.
package main

import "fmt"

type Row struct {
	ID    int
	Label string
}

// File is an immutable data file. Once written it never changes.
type File struct {
	Name string
	Rows []Row
}

// Table is one snapshot: the data files plus small delete and change files.
type Table struct {
	Snapshot int
	Data     []File
	Deletes  map[int]bool // merge-on-read: row ids a delete file removed
	Changes  map[int]Row  // merge-on-read: corrected rows in small files, newest wins
	Deltas   int          // count of small delete and change files
}

// UpdateCOW copies every row of the file holding id into a new file.
func (t *Table) UpdateCOW(id int, label string) (rowsWritten int) {
	for k, f := range t.Data {
		for _, r := range f.Rows {
			if r.ID != id {
				continue
			}
			nf := File{Name: f.Name + "'"} // a new file: the old one is immutable
			for _, x := range f.Rows {     // copy every row, fix the one
				if x.ID == id {
					x.Label = label
				}
				nf.Rows = append(nf.Rows, x)
			}
			t.Data[k] = nf // the new snapshot lists the new file
			t.Snapshot++
			return len(nf.Rows)
		}
	}
	return 0
}

// UpdateMOR leaves the data file alone and writes two tiny files.
func (t *Table) UpdateMOR(id int, label string) (rowsWritten int) {
	t.Deletes[id] = true                      // delete file: "row id is gone"
	t.Changes[id] = Row{ID: id, Label: label} // change file: the fixed row
	t.Deltas += 2
	t.Snapshot++
	return 2
}

// Read returns the current rows. Merge-on-read skips deleted ids and adds
// the corrected rows: every read pays for the merge.
func (t *Table) Read() (rows []Row, filesOpened int) {
	for _, f := range t.Data {
		filesOpened++
		for _, r := range f.Rows {
			if t.Deletes[r.ID] {
				continue // named by a delete file
			}
			rows = append(rows, r)
		}
	}
	filesOpened += t.Deltas
	for id := 0; id < 16; id++ { // add each corrected row once
		if r, ok := t.Changes[id]; ok {
			rows = append(rows, r)
		}
	}
	return rows, filesOpened
}

// Compact folds every delta into clean data files and clears the deltas.
func (t *Table) Compact() {
	rows, _ := t.Read()
	t.Data = []File{{Name: "A2", Rows: rows[:8]}, {Name: "B2", Rows: rows[8:]}}
	t.Deletes, t.Changes, t.Deltas = map[int]bool{}, map[int]Row{}, 0
	t.Snapshot++
}

func fresh() *Table {
	t := &Table{Snapshot: 1, Deletes: map[int]bool{}, Changes: map[int]Row{}}
	for _, n := range []string{"A", "B"} {
		f := File{Name: n}
		for k := 0; k < 8; k++ {
			id := len(t.Data)*8 + k
			f.Rows = append(f.Rows, Row{ID: id, Label: "dex"})
		}
		t.Data = append(t.Data, f)
	}
	t.Data[0].Rows[3].Label = "wrong"
	return t
}

func main() {
	t := fresh()
	w := t.UpdateCOW(3, "dex")
	_, opened := t.Read()
	fmt.Printf("copy-on-write: fix 1 row, rows written %d, snapshot s%d, files opened per read %d\n", w, t.Snapshot, opened)
	total := 0
	for k := 0; k < 3; k++ { // changes every minute, each touching file A
		total += t.UpdateCOW(1+k, "dex")
	}
	fmt.Printf("copy-on-write: 3 more changes rewrite file A 3 times, rows written %d\n", total)

	t = fresh()
	w = t.UpdateMOR(3, "dex")
	rows, opened := t.Read()
	fmt.Printf("merge-on-read: fix 1 row, rows written %d, snapshot s%d, files opened per read %d, rows %d\n", w, t.Snapshot, opened, len(rows))
	for k := 0; k < 47; k++ { // weeks of changes, no compaction
		t.UpdateMOR(8+k%8, "dex")
	}
	_, opened = t.Read()
	fmt.Printf("merge-on-read, no compaction: small files %d, files opened per read %d\n", t.Deltas, opened)
	t.Compact()
	rows, opened = t.Read()
	fmt.Printf("after compaction: small files %d, files opened per read %d, rows %d\n", t.Deltas, opened, len(rows))
}
