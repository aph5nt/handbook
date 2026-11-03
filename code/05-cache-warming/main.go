// Cache warming: after a restart the graph database's pages are cold.
// A warmer reads the structure first, then values newest first, and keeps
// running as new blocks arrive. A warmer that stops goes cold again.
package main

import (
	"fmt"
	"slices"
)

// Cache holds a limited number of value pages, one per day of blocks.
type Cache struct {
	limit     int
	structure bool
	values    []int // days held, oldest first
}

func (c *Cache) warmStructure() { c.structure = true }

func (c *Cache) has(day int) bool {
	return slices.Contains(c.values, day)
}

func (c *Cache) warmValue(day int) {
	if c.has(day) {
		return
	}
	if len(c.values) == c.limit {
		oldest := slices.Index(c.values, slices.Min(c.values))
		c.values = slices.Delete(c.values, oldest, oldest+1) // the oldest day falls out, and that is fine
	}
	c.values = append(c.values, day)
}

// search needs the structure pages and the values of one day.
func (c *Cache) search(day int) string {
	switch {
	case !c.structure:
		return "cold: structure and values from disk"
	case !c.has(day):
		return "half cold: values from disk"
	}
	return "warm: all from memory"
}

func main() {
	const today = 100
	c := &Cache{limit: 3}
	fmt.Println("restart, no warmer: search about today ->", c.search(today))
	c.warmStructure() // pass 1: every search needs it
	fmt.Println("warmer pass 1, structure: search ->", c.search(today))
	for d := today; d > today-3; d-- { // pass 2: newest first, as far as memory allows
		c.warmValue(d)
	}
	fmt.Printf("warmer pass 2, values newest first %v: search -> %s\n", c.values, c.search(today))

	once := *c // this warmer stops here
	once.values = append([]int{}, c.values...)
	for d := today + 1; d <= today+2; d++ { // the chain keeps moving
		c.warmValue(d) // the warmer that keeps running reads new days too
	}
	fmt.Println("two days later:")
	fmt.Printf("  warm once, then stop %v: search about today -> %s\n", once.values, once.search(today+2))
	fmt.Printf("  warmer keeps running %v: search about today -> %s\n", c.values, c.search(today+2))
	fmt.Printf("  warmer keeps running: search about old day %d -> %s\n", today-5, c.search(today-5))
}
