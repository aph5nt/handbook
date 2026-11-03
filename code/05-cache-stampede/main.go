// A cache stampede with real goroutines: 50 readers, one expired key. Without
// protection, 50 queries. With a lock per key, 1. With a stale copy, nobody
// waits. Jitter spreads 10 keys that were set together.
package main

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

const readers = 50

type Cache struct {
	mu      sync.Mutex
	val     string
	fresh   bool
	stale   string
	loading bool
	done    chan struct{}
	queries atomic.Int32
	waited  atomic.Int32
}

func (c *Cache) query() string {
	c.queries.Add(1)
	time.Sleep(50 * time.Millisecond) // the home page query
	return "home page"
}

// ReadNaive: every reader that misses rebuilds the page itself.
func (c *Cache) ReadNaive() string {
	c.mu.Lock()
	if c.fresh {
		defer c.mu.Unlock()
		return c.val
	}
	c.mu.Unlock()
	v := c.query()
	c.mu.Lock()
	c.val, c.fresh = v, true
	c.mu.Unlock()
	return v
}

// ReadLocked: one reader rebuilds, the others wait for its result.
func (c *Cache) ReadLocked(serveStale bool) string {
	c.mu.Lock()
	if c.fresh {
		defer c.mu.Unlock()
		return c.val
	}
	if c.loading { // someone holds the lock
		if serveStale {
			defer c.mu.Unlock()
			return c.stale // the old page, at once
		}
		done := c.done
		c.mu.Unlock()
		c.waited.Add(1)
		<-done
		return c.val
	}
	c.loading, c.done = true, make(chan struct{})
	c.mu.Unlock()
	v := c.query()
	c.mu.Lock()
	c.val, c.fresh, c.loading = v, true, false
	close(c.done)
	c.mu.Unlock()
	return v
}

func burst(read func() string) {
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; read() }()
	}
	close(start)
	wg.Wait()
}

func main() {
	c := &Cache{}
	burst(c.ReadNaive)
	fmt.Printf("no protection: %d readers miss, %d queries at the database\n", readers, c.queries.Load())

	c = &Cache{}
	burst(func() string { return c.ReadLocked(false) })
	fmt.Printf("one lock per key: %d query, %d readers waited for it\n", c.queries.Load(), c.waited.Load())

	c = &Cache{stale: "old home page"}
	burst(func() string { return c.ReadLocked(true) })
	fmt.Printf("stale copy while one refreshes: %d query, %d readers waited\n", c.queries.Load(), c.waited.Load())

	r := rand.New(rand.NewSource(7))
	same, spread := map[int]int{}, map[int]int{}
	for k := 0; k < 10; k++ {
		same[60]++
		spread[60+r.Intn(13)-6]++ // 60 s plus or minus 6
	}
	fmt.Printf("10 keys, TTL exactly 60 s: expiries per second %v\n", same)
	fmt.Printf("10 keys, TTL 60 s +/- 6: expiries per second %v\n", spread)
}
