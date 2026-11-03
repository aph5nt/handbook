// Follow page 7 of a Postgres table through the layers: the buffer pool
// (with its clock-sweep eviction), the OS page cache, and the disk.
package main

import "fmt"

// Pool is the buffer pool: N buffers and a clock hand. A read adds to a
// buffer's usage count. The hand lowers counts and takes the first at zero.
type Pool struct {
	pages  []int
	usage  []int
	hand   int
	lookup map[int]int
}

func NewPool(n int) *Pool {
	p := &Pool{pages: make([]int, n), usage: make([]int, n), lookup: map[int]int{}}
	for i := range p.pages {
		p.pages[i] = -1
	}
	return p
}

func (p *Pool) Get(page int) bool {
	if i, ok := p.lookup[page]; ok {
		if p.usage[i] < 5 { // the usage count caps at 5
			p.usage[i]++
		}
		return true
	}
	return false
}

// Put runs the clock sweep to find a victim.
func (p *Pool) Put(page int) {
	for {
		i := p.hand
		p.hand = (p.hand + 1) % len(p.pages)
		if p.usage[i] > 0 {
			p.usage[i]-- // a second chance
			continue
		}
		if p.pages[i] >= 0 {
			delete(p.lookup, p.pages[i])
		}
		p.pages[i], p.usage[i] = page, 1
		p.lookup[page] = i
		return
	}
}

type DB struct {
	pool      *Pool
	pageCache map[int]bool // the operating system's copy
	diskReads int
}

// Read walks the layers and says where the page came from.
func (d *DB) Read(page int) string {
	if d.pool.Get(page) {
		return "buffer pool: no system call, no disk"
	}
	from := "OS page cache: one system call, no disk"
	if !d.pageCache[page] {
		d.diskReads++
		d.pageCache[page] = true // the disk read lands in the page cache
		from = "disk"
	}
	d.pool.Put(page) // Postgres copies it in: now in memory twice
	return from
}

func main() {
	d := &DB{pool: NewPool(4), pageCache: map[int]bool{}}
	fmt.Println("query 1, page 7:", d.Read(7))
	_, inPool := d.pool.lookup[7]
	fmt.Printf("  in the pool=%v, in the page cache=%v: two copies\n", inPool, d.pageCache[7])
	fmt.Println("query 2, page 7:", d.Read(7))
	for _, other := range []int{20, 21, 22, 23, 24, 25, 26, 27} { // other pages push it out
		d.Read(other)
	}
	_, inPool = d.pool.lookup[7]
	fmt.Printf("after 8 other pages: page 7 in the pool=%v, in the page cache=%v\n", inPool, d.pageCache[7])
	fmt.Println("query 3, page 7:", d.Read(7))
	fmt.Printf("disk reads in total: %d (page 7 once, the 8 others once each)\n\n", d.diskReads)

	const ram = 64 // GB
	for _, split := range []struct {
		name string
		pct  int
	}{{"Postgres guide, shared_buffers about a quarter", 25}, {"buffer pool at 90 percent", 90}} {
		pool := ram * split.pct / 100
		left := ram - pool
		const other = 8 // connections, sorts, the OS itself
		state := "the page cache has room"
		if left <= other {
			state = "no page cache left, the OS starts to swap"
		}
		fmt.Printf("%-48s pool %2d GB, left %2d GB, other needs %d GB: %s\n", split.name, pool, left, other, state)
	}
}
