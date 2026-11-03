// Replays the clean and onion lesson: the graph sync's rings.
// Entities merge a link by its key. The use case syncs a block range
// through interfaces it owns. Adapters implement them. A lint checks that
// every import points inward, and a count shows the cost of rings for CRUD.
package main

import (
	"fmt"
	"sort"
)

// ---- ring 0: entities. True with no computer at all.

// Link says money moved from one address to another.
type Link struct{ From, To string }

// Key names a link. Two rows with one key are one link.
func (l Link) Key() string { return l.From + "->" + l.To }

// MergeLink adds a link once. A second row with the same key finds it.
func MergeLink(g map[string]Link, l Link) (added bool) {
	if _, seen := g[l.Key()]; seen {
		return false // merged, never added twice
	}
	g[l.Key()] = l
	return true
}

// ---- ring 1: use cases. They own the interfaces they need.

// Row is one transfer row from a block.
type Row struct {
	Block    int
	From, To string
	Amount   int
}

type RowReader interface{ Rows(from, to int) []Row }
type LinkWriter interface {
	Merge(links []Link) (added int)
}

// SyncRange is the use case "sync a block range".
func SyncRange(r RowReader, w LinkWriter, from, to int) (rows, added int) {
	var links []Link
	for _, row := range r.Rows(from, to) {
		links = append(links, Link{row.From, row.To})
	}
	return len(links), w.Merge(links)
}

// ---- ring 2: interface adapters. Here, in-memory ones for the test.

type memReader struct{ rows []Row } // stands in for the StarRocks reader

func (m memReader) Rows(from, to int) []Row {
	var out []Row
	for _, r := range m.rows {
		if r.Block >= from && r.Block <= to {
			out = append(out, r)
		}
	}
	return out
}

type memWriter struct{ graph map[string]Link } // stands in for the DozerDB writer

func (m memWriter) Merge(links []Link) int {
	added := 0
	for _, l := range links {
		if MergeLink(m.graph, l) {
			added++
		}
	}
	return added
}

// ---- the dependency rule, as a lint over imports.

var ring = map[string]int{"entities": 0, "usecases": 1, "adapters": 2, "drivers": 3}

type Import struct{ From, To string }

// Lint fails every import from an inner ring to an outer one.
func Lint(imports []Import) []string {
	var bad []string
	for _, im := range imports {
		if ring[im.To] > ring[im.From] { // points outward
			bad = append(bad, im.From+" imports "+im.To)
		}
	}
	sort.Strings(bad)
	return bad
}

func main() {
	rows := []Row{
		{100, "0xA", "0xB", 5}, {100, "0xB", "0xC", 2},
		{101, "0xA", "0xB", 3}, {102, "0xC", "0xA", 1}, {102, "0xB", "0xC", 4},
	}
	w := memWriter{graph: map[string]Link{}}

	fmt.Println("entities and use case: sync blocks 100 to 102")
	n, added := SyncRange(memReader{rows}, w, 100, 102)
	fmt.Printf("  rows read: %d, links added: %d, links in graph: %d\n", n, added, len(w.graph))
	fmt.Printf("  rows 0xA->0xB: 2, links 0xA->0xB: 1 (key %q)\n", Link{"0xA", "0xB"}.Key())
	_, again := SyncRange(memReader{rows}, w, 100, 102)
	fmt.Printf("  same range again: links added %d, links in graph %d (never added twice)\n", again, len(w.graph))

	good := []Import{
		{"usecases", "entities"},
		{"adapters", "usecases"}, {"adapters", "entities"},
		{"drivers", "adapters"},
	}
	fmt.Printf("lint, every arrow in: %d imports, %d refused\n", len(good), len(Lint(good)))

	bad := append(good, Import{"usecases", "drivers"}) // the DozerDB driver type in the use case
	fmt.Printf("lint, one arrow out: %d imports, refused %v\n", len(bad), Lint(bad))

	fields, rings := 5, 4 // a CRUD app that edits one table of 5 fields
	fmt.Printf("rings for CRUD: %d fields x %d rings = %d field copies, %d mappers, rules protected: 0\n",
		fields, rings, fields*rings, rings-1)
}
