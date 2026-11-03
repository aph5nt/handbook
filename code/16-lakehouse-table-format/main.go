// A table format over files, in miniature: snapshots list the files that
// make up the table, a commit swaps the current pointer in one step, and
// old snapshots keep their files alive until they are expired.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Snapshot struct {
	ID    string
	Files []string
}

type Table struct {
	storage   map[string]bool // every file in object storage
	snapshots []Snapshot
	current   int // index of the current snapshot
}

// Write puts files in storage. No reader sees them yet.
func (t *Table) Write(files ...string) {
	for _, f := range files {
		t.storage[f] = true
	}
}

// Commit adds a snapshot and moves the pointer in one step: atomic.
func (t *Table) Commit(id string, files []string) {
	t.snapshots = append(t.snapshots, Snapshot{id, files})
	t.current = len(t.snapshots) - 1
}

// Scan returns the files of the current snapshot: what every engine reads.
func (t *Table) Scan() []string { return t.snapshots[t.current].Files }

// AsOf returns the files of an older snapshot: time travel.
func (t *Table) AsOf(id string) []string {
	for _, s := range t.snapshots {
		if s.ID == id {
			return s.Files
		}
	}
	return nil
}

// Expire drops snapshots other than the current one, then deletes files
// that no remaining snapshot lists, orphans included.
func (t *Table) Expire() (deleted int) {
	t.snapshots = t.snapshots[t.current:]
	t.current = 0
	keep := map[string]bool{}
	for _, f := range t.Scan() {
		keep[f] = true
	}
	for f := range t.storage {
		if !keep[f] {
			delete(t.storage, f)
			deleted++
		}
	}
	return deleted
}

func (t *Table) show(step string) {
	fmt.Printf("%-28s current %s reads %-44s storage %d files\n",
		step, t.snapshots[t.current].ID, strings.Join(t.Scan(), ","), len(t.storage))
}

func main() {
	t := &Table{storage: map[string]bool{}}
	t.Write("part-0001", "part-0002")
	t.Commit("s1", []string{"part-0001", "part-0002"})
	t.show("files land, s1")
	t.Write("part-0003", "part-0004")
	t.show("Spark wrote 0003 0004, no commit")
	t.Commit("s2", []string{"part-0001", "part-0002", "part-0003", "part-0004"})
	t.show("commit s2")
	t.Write("part-0005") // the job crashes before its commit
	t.show("0005 written, job crashed")
	fmt.Printf("%-28s s1 reads %s\n", "time travel as of s1", strings.Join(t.AsOf("s1"), ","))
	t.Write("part-0006")
	t.Commit("s3", []string{"part-0006"})
	t.show("compaction into 0006, s3")
	fmt.Printf("%-28s snapshots %d, storage %d files, current needs 1\n", "never expired", len(t.snapshots), len(t.storage))
	n := t.Expire()
	t.show(fmt.Sprintf("expire: %d files deleted", n))

	// the swamp: no table format, the folder listing is the table
	dir, err := os.MkdirTemp("", "swamp")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	_ = WriteFile(dir, "part-0001", []byte("swap\nswap\n"))
	_ = WriteFile(dir, "part-0002", []byte("swap\nswap\n"))
	_ = WriteFile(dir, "part-0003", []byte("swap\n")) // Spark is still writing: 1 of 2 rows so far
	rows, _ := ReadFolder(dir)
	fmt.Printf("swamp: Trino lists the folder and counts %d rows, the true table has 4 complete + 2 in flight\n", rows)
}

// no table format: the folder listing is the table
func ReadFolder(dir string) (rows int, err error) {
	entries, err := os.ReadDir(dir) // whatever is there right now
	if err != nil {
		return 0, err
	}
	for _, e := range entries {
		n, err := countRows(filepath.Join(dir, e.Name()))
		if err != nil {
			return 0, err // a half-written file fails, or reads short
		}
		rows += n
	}
	return rows, nil
}

// a writer creates the file in place: readers can see it mid-write
func WriteFile(dir, name string, data []byte) error {
	return os.WriteFile(filepath.Join(dir, name), data, 0o644)
}

func countRows(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strings.Count(string(b), "\n"), nil
}
