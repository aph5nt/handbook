// Write-around: a backfill writes the database only, so the hot set
// of six live addresses stays in a six-slot LRU cache.
package main

import "fmt"

// LRU with six slots. slots[0] is the most recently used.
type LRU struct {
	cap   int
	keys  []string
	vals  map[string]string
	evict int
}

func (c *LRU) Get(k string) (string, bool) {
	v, ok := c.vals[k]
	if ok {
		c.touch(k)
	}
	return v, ok
}

func (c *LRU) Set(k, v string) {
	if _, ok := c.vals[k]; !ok && len(c.keys) == c.cap {
		old := c.keys[len(c.keys)-1] // the least recently used goes
		c.keys = c.keys[:len(c.keys)-1]
		delete(c.vals, old)
		c.evict++
	}
	c.vals[k] = v
	c.touch(k)
}

func (c *LRU) Delete(k string) {
	if _, ok := c.vals[k]; ok {
		delete(c.vals, k)
		for i, x := range c.keys {
			if x == k {
				c.keys = append(c.keys[:i], c.keys[i+1:]...)
				break
			}
		}
	}
}

func (c *LRU) touch(k string) {
	for i, x := range c.keys {
		if x == k {
			c.keys = append(c.keys[:i], c.keys[i+1:]...)
			break
		}
	}
	c.keys = append([]string{k}, c.keys...)
}

var hot = []string{"A", "B", "C", "D", "E", "F"}

func read(c *LRU, db map[string]string, k string) string {
	if v, ok := c.Get(k); ok {
		return v // hit
	}
	c.Set(k, db[k]) // miss: load and keep
	return db[k]
}

func updateLabel(c *LRU, db map[string]string, k, v string) {
	db[k] = v   // the write goes to the database only
	c.Delete(k) // the fix: drop the old copy
}

func scenario(writeThrough bool) {
	db := map[string]string{}
	c := &LRU{cap: 6, vals: map[string]string{}}
	for _, k := range hot {
		db[k] = "label v1"
		c.Set(k, "label v1")
	}
	for i := 0; i < 10_000_000; i++ { // the backfill re-reads ten million old blocks
		k := fmt.Sprint("block-", i)
		db[k] = "row"
		if writeThrough {
			c.Set(k, "row") // each old row takes a slot
		}
		if i == 5 && writeThrough {
			fmt.Printf("  after 6 backfill rows the cache holds %v\n", c.keys)
		}
		if i > 1000 {
			delete(db, k) // keep the demo's memory small
		}
	}
	hits := 0
	for _, k := range hot {
		if _, ok := c.Get(k); ok {
			hits++
		}
	}
	fmt.Printf("  investigators read the six live addresses: %d of 6 hit, evictions %d\n", hits, c.evict)
}

func main() {
	fmt.Println("== backfill written through the cache ==")
	scenario(true)
	fmt.Println("== backfill written around the cache ==")
	scenario(false)

	fmt.Println("== a label update for C, written around ==")
	for _, deleteOnWrite := range []bool{false, true} {
		db := map[string]string{"C": "label v1"}
		c := &LRU{cap: 6, vals: map[string]string{}}
		c.Set("C", "label v1")
		if deleteOnWrite {
			updateLabel(c, db, "C", "label v2")
		} else {
			db["C"] = "label v2" // the bug: the old copy stays
		}
		_, hit := c.Get("C")
		v := read(c, db, "C")
		if !hit {
			v += " (miss, loaded)"
		}
		fmt.Printf("  deleteOnWrite=%v: the next reader of C gets %q\n", deleteOnWrite, v)
	}
}
