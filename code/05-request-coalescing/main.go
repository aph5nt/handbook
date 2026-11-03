// Request coalescing (singleflight), written by hand with the sync package.
// 30 requests for one cold key send 1 graph query. Then shared fate,
// the timeouts that fix it, and ten instances with a lock key.
package main

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// scale: the lesson's seconds run as hundredths of a second here.
const second = 10 * time.Millisecond

type call struct {
	done chan struct{}
	val  string
	err  error
}

// Group merges concurrent calls for the same key into one.
type Group struct {
	mu sync.Mutex
	m  map[string]*call // key -> the call in flight
}

// DoChan starts fn for key, or joins the call already in flight.
func (g *Group) DoChan(key string, fn func() (string, error)) <-chan *call {
	g.mu.Lock()
	if g.m == nil {
		g.m = map[string]*call{}
	}
	c, inFlight := g.m[key]
	if !inFlight { // the first caller starts the load
		c = &call{done: make(chan struct{})}
		g.m[key] = c
		go func() {
			c.val, c.err = fn()
			g.mu.Lock()
			delete(g.m, key) // the key is free again
			g.mu.Unlock()
			close(c.done) // every waiter gets the same result
		}()
	}
	g.mu.Unlock()
	out := make(chan *call, 1)
	go func() { <-c.done; out <- c }()
	return out
}

// Get waits for the shared result, but never past its own deadline.
func (g *Group) Get(key string, fn func() (string, error), deadline time.Duration) (string, error) {
	select {
	case c := <-g.DoChan(key, fn):
		return c.val, c.err
	case <-time.After(deadline): // this waiter gives up alone
		return "", errors.New("caller deadline")
	}
}

func burst(g *Group, n int, fn func() (string, error), deadline time.Duration) (ok, failed int) {
	var wg sync.WaitGroup
	var okN, failN atomic.Int32
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := g.Get("0x77..30ff", fn, deadline); err == nil {
				okN.Add(1)
			} else {
				failN.Add(1)
			}
		}()
	}
	wg.Wait()
	return int(okN.Load()), int(failN.Load())
}

func main() {
	var queries atomic.Int32
	query := func(d time.Duration) func() (string, error) {
		return func() (string, error) {
			queries.Add(1)
			time.Sleep(d)
			return "high", nil
		}
	}
	g := &Group{}
	ok, _ := burst(g, 30, query(5*second), 10*second)
	fmt.Printf("30 requests, cold key: graph queries=%d, answered=%d\n", queries.Load(), ok)

	// shared fate: the query hangs. The shared load has a 2 s timeout,
	// and each waiter its own 3 s deadline.
	queries.Store(0)
	hung := func() (string, error) {
		queries.Add(1)
		select {
		case <-time.After(60 * second): // the graph hangs for a minute
			return "high", nil
		case <-time.After(2 * second): // the shared load's timeout
			return "", errors.New("graph timeout after 2 s")
		}
	}
	start := time.Now()
	_, failed := burst(g, 30, hung, 3*second)
	fmt.Printf("slow graph: queries=%d, %d callers failed fast after %.0f s instead of 60 s\n",
		queries.Load(), failed, time.Since(start).Seconds()/second.Seconds())

	// ten instances, ten groups: one query each
	queries.Store(0)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); burst(&Group{}, 3, query(5*second), 10*second) }()
	}
	wg.Wait()
	fmt.Printf("10 instances without a shared lock: queries=%d\n", queries.Load())

	// a lock key in Redis: SET lock:0x77 NX EX 5. Only the winner queries.
	queries.Store(0)
	var redis sync.Map
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, taken := redis.LoadOrStore("lock:0x77..30ff", true); !taken { // SET NX
				query(1 * second)()
			} // the others wait briefly or serve a stale copy
		}()
	}
	wg.Wait()
	fmt.Printf("10 instances with SET NX lock: queries=%d\n", queries.Load())
}
