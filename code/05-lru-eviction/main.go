// LRU and LFU on four slots, with the same victims as the lesson. Both share
// the first 8 reads. Then each shows its own failure, so the totals differ.
package main

import (
	"fmt"
	"strings"
)

type slot struct {
	key   string
	count int
	last  int
}

type Cache struct {
	lfu          bool
	slots        []*slot
	t            int
	hits, misses int
}

// Victim picks the slot to throw out.
func (c *Cache) victim() int {
	best := 0
	for i, s := range c.slots {
		b := c.slots[best]
		if c.lfu { // lowest count, then read longest ago
			if s.count < b.count || (s.count == b.count && s.last < b.last) {
				best = i
			}
		} else if s.last < b.last { // LRU: read longest ago
			best = i
		}
	}
	return best
}

func (c *Cache) Read(key string) string {
	c.t++
	for _, s := range c.slots {
		if s.key == key {
			s.count++
			s.last = c.t
			c.hits++
			return "hit"
		}
	}
	c.misses++
	if len(c.slots) < 4 {
		c.slots = append(c.slots, &slot{key, 1, c.t})
		return "miss"
	}
	v := c.victim()
	out := c.slots[v].key
	c.slots[v] = &slot{key, 1, c.t}
	return "miss, threw out " + out
}

func (c *Cache) String() string {
	var b []string
	for _, s := range c.slots {
		b = append(b, fmt.Sprintf("%s(%d)", s.key, s.count))
	}
	return strings.Join(b, " ")
}

func run(name string, lfu bool, reads []string) {
	fmt.Printf("== %s ==\n", name)
	c := &Cache{lfu: lfu}
	for _, k := range reads {
		fmt.Printf("  read %s: %-20s %s\n", k, c.Read(k), c)
	}
	fmt.Printf("  hits %d, misses %d\n", c.hits, c.misses)
}

func main() {
	run("LRU", false, strings.Split("A A A B C D E A F G H I", " "))
	run("LFU", true, strings.Split("A A A B C D E A F G H", " "))
}
