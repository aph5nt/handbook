// Log compaction and tombstones: a topic that holds the latest value per key.
//
// A slice of records with fixed offsets stands in for a compacted topic. Times
// are in hours and only added up. Deterministic.
package main

import "fmt"

type Rec struct {
	Off      int
	Key, Val string // Val "" is a tombstone
	Hour     int
}

const deleteRetention = 24 // hours a tombstone is kept

// compact keeps the newest record per key and drops expired tombstones.
func compact(in []Rec, now int) []Rec {
	last := map[string]int{}
	for _, r := range in {
		last[r.Key] = r.Off
	}
	var out []Rec
	for _, r := range in {
		if last[r.Key] != r.Off {
			continue
		}
		if r.Val == "" && now-r.Hour >= deleteRetention {
			continue // the tombstone has expired
		}
		out = append(out, r)
	}
	return out
}

// fetch returns the first record at or after an offset. Gaps are normal.
func fetch(in []Rec, off int) (Rec, bool) {
	for _, r := range in {
		if r.Off >= off {
			return r, true
		}
	}
	return Rec{}, false
}

// build replays a topic into a table.
func build(in []Rec, table map[string]string) map[string]string {
	for _, r := range in {
		if r.Val == "" {
			delete(table, r.Key)
			continue
		}
		table[r.Key] = r.Val
	}
	return table
}

func main() {
	log := []Rec{{0, "A", "v1", 0}, {1, "B", "v1", 0}, {2, "A", "v2", 0}, {3, "C", "v1", 0}, {4, "B", "v2", 0}, {5, "A", "v3", 0}}
	fmt.Println("== Tab 1: a table ==")
	c := compact(log, 1)
	fmt.Printf("%d records become %d: %v\n", len(log), len(c), c)
	fmt.Printf("new reader's table: %v\n", build(c, map[string]string{}))
	r, _ := fetch(c, 1)
	fmt.Printf("a consumer asks for offset 1, gets the next existing: offset %d\n", r.Off)

	fmt.Println("== Tab 2: tombstones ==")
	offline := build(c, map[string]string{}) // the alert service reads, then goes offline
	withTomb := append(append([]Rec{}, log...), Rec{6, "B", "", 10})
	c2 := compact(withTomb, 11)
	fmt.Printf("after the tombstone, kept: %v\n", c2)
	fmt.Printf("live consumer's table: %v\n", build(c2, map[string]string{}))
	c3 := compact(withTomb, 10+deleteRetention)
	fmt.Printf("after %d hours the tombstone is gone: %v\n", deleteRetention, c3)
	fmt.Printf("offline consumer back after 3 days reads it: %v: still holds B\n", build(c3, offline))

	fmt.Println("== Tab 3: a state store ==")
	changelog := []Rec{{0, "A", "v3", 0}, {1, "C", "v1", 0}}
	fmt.Printf("new pod restored from the compacted changelog: %v\n", build(changelog, map[string]string{}))
	var timeRetained []Rec
	now := 30 * 24                                                        // hour 720: a month in
	for _, r := range []Rec{{0, "A", "v3", 29 * 24}, {1, "C", "v1", 0}} { // C was written a month ago
		// time retention instead of compaction: by age, not by key
		if now-r.Hour > 7*24 {
			continue // token C's only value is deleted too
		}
		timeRetained = append(timeRetained, r)
	}
	fmt.Printf("with 7-day time retention instead: %v: token C is missing\n", build(timeRetained, map[string]string{}))
}
