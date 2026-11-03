// An in-memory model of a B-tree over a users table: 300 keys per index page,
// 100 rows per table page. It needs no database. It counts page reads, which is
// the number EXPLAIN (BUFFERS) prints on a real engine.
package main

import (
	"fmt"
	"sort"
)

const (
	fanout      = 300
	rowsPerPage = 100
)

type Page struct {
	first []int   // the first key under each child, in an inner page
	kids  []*Page // nil for a leaf
	keys  []int   // keys, in a leaf
	next  *Page   // the leaf to the right
}

// build makes the leaves, then stacks inner levels until one root is left.
func build(n int) (root *Page, levels int) {
	var level []*Page
	var prev *Page
	for k := 0; k < n; k += fanout {
		leaf := &Page{}
		for j := k; j < k+fanout && j < n; j++ {
			leaf.keys = append(leaf.keys, j)
		}
		if prev != nil {
			prev.next = leaf
		}
		prev = leaf
		level = append(level, leaf)
	}
	levels = 1
	for len(level) > 1 {
		var up []*Page
		for i := 0; i < len(level); i += fanout {
			p := &Page{}
			for j := i; j < i+fanout && j < len(level); j++ {
				p.kids = append(p.kids, level[j])
				p.first = append(p.first, firstKey(level[j]))
			}
			up = append(up, p)
		}
		level = up
		levels++
	}
	return level[0], levels
}

func firstKey(p *Page) int {
	if p.kids == nil {
		return p.keys[0]
	}
	return p.first[0]
}

// seek walks root to leaf and returns the leaf and the pages read.
func seek(root *Page, key int) (*Page, int) {
	reads := 1
	p := root
	for p.kids != nil {
		i := sort.Search(len(p.first), func(i int) bool { return p.first[i] > key }) - 1
		if i < 0 {
			i = 0
		}
		p = p.kids[i]
		reads++
	}
	return p, reads
}

// rangeScan seeks to lo, walks the linked leaves to hi, and reads a table page per row.
func rangeScan(root *Page, lo, hi int) (rows, reads int) {
	leaf, reads := seek(root, lo)
	for leaf != nil {
		for _, k := range leaf.keys {
			if k >= lo && k <= hi {
				rows++
			}
		}
		if leaf.keys[len(leaf.keys)-1] >= hi {
			break
		}
		leaf = leaf.next
		reads++
	}
	return rows, reads + rows
}

func main() {
	const users = 1_000_000
	const ana = 425_000 // Ana's key, and her row number

	fmt.Printf("table: %d rows, %d pages. Ana is on page %d, box %d of 40\n",
		users, users/rowsPerPage, ana/rowsPerPage, ana/rowsPerPage/250)

	comparisons, pages := 0, 0
	for page := 0; page < users/rowsPerPage; page++ {
		pages++
		comparisons += rowsPerPage
	}
	fmt.Printf("scan: %d page reads, %d comparisons, 1 row returned\n", pages, comparisons)

	root, levels := build(users)
	_, reads := seek(root, ana)
	fmt.Printf("seek: %d levels, %d index pages + 1 table page = %d page reads\n", levels, reads, reads+1)

	// Three keys around a leaf boundary: 299 ends one leaf, so they straddle two.
	boundary := ana - ana%fanout + fanout - 1
	rows, r := rangeScan(root, boundary, boundary+2)
	fmt.Printf("range: %d rows across two leaves = %d page reads\n", rows, r)

	depth := 1
	for n := 1_000_000_000 / fanout; n > 1; n = (n + fanout - 1) / fanout {
		depth++
	}
	fmt.Printf("a billion keys need %d levels\n", depth)

	const report = 600_000
	fmt.Printf("60 percent report: %d seeks x 4 = %d page reads, one sweep = %d\n", report, report*4, users/rowsPerPage)
}
