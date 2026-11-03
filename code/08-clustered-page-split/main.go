// An in-memory model of the leaf pages of a clustered index: four rows per
// page in the story, 100 rows per page in the bulk run. It needs no database.
// It proves where a key lands, when a page splits, and how full pages end up.
package main

import (
	"fmt"
	"math/rand"
	"sort"
)

type Tree struct {
	cap     int
	pages   [][]uint64 // leaf pages, in key order
	splits  int
	appends int
	writes  int // pages written, counting the parent on a split or a new page
}

// Insert finds the leaf by key, then appends, adds a page, or splits.
func (t *Tree) Insert(k uint64) string {
	if len(t.pages) == 0 {
		t.pages = [][]uint64{{k}}
		t.writes++
		return "first page"
	}
	// The last page whose first key is <= k.
	p := sort.Search(len(t.pages), func(i int) bool { return t.pages[i][0] > k }) - 1
	if p < 0 {
		p = 0
	}
	if len(t.pages[p]) < t.cap {
		t.place(p, k)
		t.writes++
		return "room in the page"
	}
	last := t.pages[p][len(t.pages[p])-1]
	if p == len(t.pages)-1 && k > last {
		// Bigger than every key: a fresh page, nothing moves.
		t.pages = append(t.pages, []uint64{k})
		t.appends++
		t.writes += 2 // the new page and the parent
		return "new page at the end"
	}
	// Full page in the middle: half the rows move to a new page.
	half := t.cap / 2
	right := append([]uint64{}, t.pages[p][half:]...)
	t.pages[p] = t.pages[p][:half:half]
	t.pages = append(t.pages[:p+1], append([][]uint64{right}, t.pages[p+1:]...)...)
	t.splits++
	t.writes += 3 // the old page, the new page and the parent
	if k >= right[0] {
		t.place(p+1, k)
	} else {
		t.place(p, k)
	}
	return "split"
}

func (t *Tree) place(p int, k uint64) {
	pg := t.pages[p]
	i := sort.Search(len(pg), func(i int) bool { return pg[i] > k })
	pg = append(pg, 0)
	copy(pg[i+1:], pg[i:])
	pg[i] = k
	t.pages[p] = pg
}

func (t *Tree) fill() float64 {
	n := 0
	for _, p := range t.pages {
		n += len(p)
	}
	return 100 * float64(n) / float64(len(t.pages)*t.cap)
}

func main() {
	fmt.Println("Story: sequence keys, four rows per page")
	s := &Tree{cap: 4}
	for k := uint64(101); k <= 112; k++ {
		s.Insert(k)
	}
	for k := uint64(113); k <= 117; k++ {
		fmt.Printf("  insert %d: %s\n", k, s.Insert(k))
	}

	fmt.Println("Story: random keys, four rows per page")
	u := &Tree{cap: 4, pages: [][]uint64{
		{0x0b, 0x1a, 0x2c, 0x3f}, {0x41, 0x5d, 0x6e, 0x7a},
		{0x8c, 0x9b, 0xb4, 0xc7}, {0xd2, 0xe5, 0xf0},
	}}
	fmt.Printf("  insert 4c: %s, %d pages written\n", u.Insert(0x4c), u.writes)
	fmt.Printf("  page 2 is now %x, the new page is %x\n", u.pages[1], u.pages[2])
	fmt.Printf("  insert 52: %s\n", u.Insert(0x52))

	fmt.Println("Bulk: one million rows, 100 rows per page")
	const n = 1_000_000
	seq, rnd := &Tree{cap: 100}, &Tree{cap: 100}
	r := rand.New(rand.NewSource(1))
	for i := 0; i < n; i++ {
		seq.Insert(uint64(i))
		rnd.Insert(r.Uint64())
	}
	for _, x := range []struct {
		name string
		t    *Tree
	}{{"sequence", seq}, {"random", rnd}} {
		fmt.Printf("  %-9s pages %6d, splits %6d, average fill %3.0f%%\n", x.name, len(x.t.pages), x.t.splits, x.t.fill())
	}
}
