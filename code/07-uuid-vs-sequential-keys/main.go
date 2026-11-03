// UUID versus sequential keys: one B-tree index, leaf pages of 100 keys, and
// memory that holds 2 pages. A counter appends at the right edge. Random
// UUIDv4 keys land anywhere, read pages from disk and split full pages.
// UUIDv7 starts with the time, so it appends like a counter again.
// Run: go run .
package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/rand"
	"sort"
)

// Page is one leaf page: the key range it covers and its sorted keys.
type Page struct {
	lo, hi int
	keys   []int
}

// Index is the leaf level of a B-tree plus the pages cached in memory.
type Index struct {
	pages     []*Page // in key order
	capacity  int     // keys per page
	cache     []*Page // most recently used first
	cacheSize int
	align     int // a split boundary is rounded down to this
	diskReads int
	splits    int
}

// find returns the page whose key range holds key.
func (ix *Index) find(key int) (int, *Page) {
	i := sort.Search(len(ix.pages), func(i int) bool { return ix.pages[i].hi >= key })
	return i, ix.pages[i]
}

// touch brings a page into memory. A page not in memory is a disk read,
// and the least recently used page is pushed out.
func (ix *Index) touch(p *Page) (diskRead bool) {
	for i, c := range ix.cache {
		if c == p {
			ix.cache = append([]*Page{p}, append(ix.cache[:i:i], ix.cache[i+1:]...)...)
			return false
		}
	}
	ix.diskReads++
	ix.cache = append([]*Page{p}, ix.cache...)
	if len(ix.cache) > ix.cacheSize {
		ix.cache = ix.cache[:ix.cacheSize]
	}
	return true
}

func (ix *Index) remember(p *Page) {
	ix.cache = append([]*Page{p}, ix.cache...)
	if len(ix.cache) > ix.cacheSize {
		ix.cache = ix.cache[:ix.cacheSize]
	}
}

// Insert puts a key on the page where it belongs in key order.
func (ix *Index) Insert(key int) (p *Page, diskRead, split bool) {
	i, p := ix.find(key)
	diskRead = ix.touch(p)
	if len(p.keys) == ix.capacity {
		last := i == len(ix.pages)-1 && key > p.keys[len(p.keys)-1]
		if last {
			// Right edge: start a new page. The full page stays full.
			np := &Page{lo: key, hi: p.hi}
			p.hi = key - 1
			ix.pages = append(ix.pages, np)
			ix.remember(np)
			p = np
		} else {
			// Middle: split the full page into two half-empty pages.
			mid := p.keys[len(p.keys)/2] / ix.align * ix.align
			cut := sort.SearchInts(p.keys, mid)
			right := &Page{lo: mid, hi: p.hi, keys: append([]int{}, p.keys[cut:]...)}
			p.hi, p.keys = mid-1, p.keys[:cut]
			ix.pages = append(ix.pages[:i+1], append([]*Page{right}, ix.pages[i+1:]...)...)
			ix.splits++
			split = true
			if key >= mid {
				p = right
			}
		}
	}
	at := sort.SearchInts(p.keys, key)
	p.keys = append(p.keys[:at], append([]int{key}, p.keys[at:]...)...)
	return p, diskRead, split
}

// Fill is how full the pages are on average, in percent.
func (ix *Index) Fill() int {
	keys := 0
	for _, p := range ix.pages {
		keys += len(p.keys)
	}
	return keys * 100 / (len(ix.pages) * ix.capacity)
}

func seqLabel(p *Page) string {
	if len(p.keys) == 1 {
		return fmt.Sprint(p.keys[0])
	}
	return fmt.Sprintf("%d-%d", p.keys[0], p.keys[len(p.keys)-1])
}

func hexLabel(p *Page) string { return fmt.Sprintf("%02x-%02x", p.lo>>8, p.hi>>8) }

func sequential() {
	fmt.Println("== key = BIGINT identity")
	ix := &Index{capacity: 100, cacheSize: 2, align: 1}
	for lo := 1; lo <= 601; lo += 100 {
		p := &Page{lo: lo, hi: lo + 99}
		for k := lo; k <= lo+99 && k <= 660; k++ {
			p.keys = append(p.keys, k)
		}
		ix.pages = append(ix.pages, p)
	}
	last := ix.pages[len(ix.pages)-1]
	last.hi = math.MaxInt
	ix.cache = []*Page{last, ix.pages[len(ix.pages)-2]}
	lastLabel := seqLabel(last)
	fmt.Printf("start: %d pages, last page %s holds %d keys\n", len(ix.pages), lastLabel, len(last.keys))

	p, read, _ := ix.Insert(661) // the next number from the counter
	fmt.Printf("insert 661: page %s, disk read %v\n", seqLabel(p), read)

	for k := 662; k <= 700; k++ {
		ix.Insert(k)
	}
	label := seqLabel(p)
	fmt.Printf("inserts 662-700: page %s holds %d keys, disk reads %d\n", label, len(p.keys), ix.diskReads)

	np, read, split := ix.Insert(701)
	fmt.Printf("insert 701: page %s is full, new page %s at the right edge, disk read %v, split %v\n", seqLabel(p), seqLabel(np), read, split)

	first, later := 1005, 1405 // two order ids a customer sees a week apart
	fmt.Printf("/orders/%d on 1 March, /orders/%d on 8 March: about %d orders a week\n", first, later, later-first)
}

// newUUIDv4 is 122 random bits: no database needed, unique anyway.
func newUUIDv4(r io.Reader) [16]byte {
	var u [16]byte
	io.ReadFull(r, u[:])
	u[6] = u[6]&0x0f | 0x40 // version 4
	u[8] = u[8]&0x3f | 0x80
	return u
}

// newUUIDv7 starts with a 48-bit millisecond timestamp, then random bits.
func newUUIDv7(ms int64, r io.Reader) [16]byte {
	var u [16]byte
	io.ReadFull(r, u[6:])
	var t [8]byte
	binary.BigEndian.PutUint64(t[:], uint64(ms))
	copy(u[:6], t[2:])
	u[6] = u[6]&0x0f | 0x70 // version 7
	u[8] = u[8]&0x3f | 0x80
	return u
}

func uuidString(u [16]byte) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:])
}

// key is the first 8 bytes of a UUID: the order the index sorts by.
func key(u [16]byte) int { return int(binary.BigEndian.Uint64(u[:8]) >> 1) }

func random() {
	fmt.Println("== key = random UUIDv4, shown by the first two hex digits")
	rnd := rand.New(rand.NewSource(7))
	fmt.Printf("phone, offline, makes its own id: %s\n", uuidString(newUUIDv4(rnd)))

	// Eight pages of 32 first-byte values each. Page e0-ff is full.
	ix := &Index{capacity: 100, cacheSize: 2, align: 256}
	for lo := 0; lo < 0x10000; lo += 0x2000 {
		p := &Page{lo: lo, hi: lo + 0x1fff}
		n := 60
		if lo == 0xe000 {
			n = 100
		}
		for i := 0; i < n; i++ {
			p.keys = append(p.keys, lo+i*82)
		}
		ix.pages = append(ix.pages, p)
	}
	ix.cache = []*Page{ix.pages[2], ix.pages[6]} // 40-5f and c0-df
	a, b := hexLabel(ix.cache[0]), hexLabel(ix.cache[1])
	fmt.Printf("start: %d pages, 2 in memory: %s and %s\n", len(ix.pages), a, b)

	for _, k := range []int{0x9f12, 0x1c34} {
		p, read, _ := ix.Insert(k)
		label, a, b := hexLabel(p), hexLabel(ix.cache[0]), hexLabel(ix.cache[1])
		fmt.Printf("insert %02x..: page %s, disk read %v, now in memory %s and %s\n", k>>8, label, read, a, b)
	}

	full := ix.pages[7]
	n := len(full.keys)
	p, read, split := ix.Insert(0xe4a0)
	right := ix.pages[8]
	left, rightLabel := hexLabel(p), hexLabel(right)
	fmt.Printf("insert e4..: page e0-ff held %d keys, disk read %v, split %v: %s holds %d, %s holds %d\n", n, read, split, left, len(p.keys), rightLabel, len(right.keys))
}

// bulk inserts 10,000 keys into an empty index and counts the cost.
func bulk(name string, next func(i int) int) {
	ix := &Index{pages: []*Page{{lo: math.MinInt, hi: math.MaxInt}}, capacity: 100, cacheSize: 2, align: 1}
	ix.cache = []*Page{ix.pages[0]}
	for i := 0; i < 10000; i++ {
		ix.Insert(next(i))
	}
	fill := ix.Fill()
	fmt.Printf("%s: 10000 inserts, %d disk reads, %d splits, %d pages, %d%% full\n", name, ix.diskReads, ix.splits, len(ix.pages), fill)
}

func main() {
	sequential()
	fmt.Println()
	random()
	fmt.Println()
	fmt.Println("== the same 10,000 inserts, 2 pages in memory")
	bulk("BIGINT counter", func(i int) int { return i + 1 })
	r4 := rand.New(rand.NewSource(4))
	bulk("UUIDv4", func(int) int { return key(newUUIDv4(r4)) })
	r7 := rand.New(rand.NewSource(7))
	start := int64(1730600000000) // a millisecond clock
	bulk("UUIDv7", func(i int) int { return key(newUUIDv7(start+int64(i), r7)) })
	a, b := newUUIDv7(start, r7), newUUIDv7(start+1, r7)
	sa, sb, less := uuidString(a), uuidString(b), key(a) < key(b)
	fmt.Printf("UUIDv7 %s < %s: %v\n", sa, sb, less)
}
