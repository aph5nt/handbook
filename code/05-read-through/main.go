// Read-through versus cache-aside, replaying the risk-service story.
// A fake clock keeps the run instant and the numbers exact.
package main

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var now = time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)

type item struct {
	val     string
	err     error
	expires time.Time
}

// ReadThrough owns the loader. Callers only call Get.
type ReadThrough struct {
	mu          sync.Mutex
	ttl         time.Duration
	load        func(key string) (string, error)
	items       map[string]item
	inflight    map[string]*call
	cacheErrors bool // the bug: store a failed load for the full TTL
}

type call struct {
	wg  sync.WaitGroup
	val string
	err error
}

func (c *ReadThrough) Get(key string) (string, error) {
	c.mu.Lock()
	if it, ok := c.items[key]; ok && now.Before(it.expires) { // hit and still fresh
		c.mu.Unlock()
		return it.val, it.err
	}
	if cl, ok := c.inflight[key]; ok { // a load for this key is running: wait for it
		c.mu.Unlock()
		cl.wg.Wait()
		return cl.val, cl.err
	}
	cl := &call{}
	cl.wg.Add(1)
	c.inflight[key] = cl
	c.mu.Unlock()

	cl.val, cl.err = c.load(key) // miss: the loader runs once per key

	c.mu.Lock()
	delete(c.inflight, key)
	if cl.err == nil || c.cacheErrors { // the fix: never store a failed load
		c.items[key] = item{cl.val, cl.err, now.Add(c.ttl)}
	}
	c.mu.Unlock()
	cl.wg.Done()
	return cl.val, cl.err
}

func main() {
	const addr = "0x77..30ff"
	graphCalls := 0
	graphDown := false
	var gate chan struct{}
	loader := func(key string) (string, error) {
		graphCalls++
		if gate != nil {
			<-gate // hold the load so other callers join it
		}
		if graphDown {
			return "", errors.New("graph timeout")
		}
		return "high", nil
	}

	fmt.Println("== cache-aside: the app does get, load, set ==")
	runRiskQuery := loader
	cache := map[string]item{} // the app owns this map
	screen := func(addr string) string {
		if it, ok := cache[addr]; ok && now.Before(it.expires) {
			return it.val // hit
		}
		v, _ := runRiskQuery(addr) // the app queries the graph itself
		cache[addr] = item{val: v, expires: now.Add(60 * time.Second)}
		return v // the app filled the cache
	}
	fmt.Println("read 1:", screen(addr), "graph calls:", graphCalls)
	fmt.Println("read 2:", screen(addr), "graph calls:", graphCalls)

	fmt.Println("\n== read-through: one loader inside the cache ==")
	graphCalls = 0
	rt := &ReadThrough{ttl: 60 * time.Second, load: runRiskQuery, items: map[string]item{}, inflight: map[string]*call{}}
	v, _ := rt.Get(addr)
	fmt.Println("Get 1:", v, "graph calls:", graphCalls)
	v, _ = rt.Get(addr)
	fmt.Println("Get 2:", v, "graph calls:", graphCalls)

	now = now.Add(61 * time.Second) // the key expires
	gate = make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); rt.Get(addr) }()
	}
	time.Sleep(50 * time.Millisecond) // let all three callers arrive
	close(gate)
	wg.Wait()
	gate = nil
	fmt.Println("3 callers after expiry, graph calls now:", graphCalls, "(one load for all three)")

	for _, bug := range []bool{true, false} {
		now = now.Add(61 * time.Second)
		rt.cacheErrors = bug
		graphDown = true
		_, err := rt.Get(addr) // the loader times out
		graphDown = false      // the graph recovers
		now = now.Add(2 * time.Second)
		v, err2 := rt.Get(addr)
		fmt.Printf("\ncacheErrors=%v: first Get err=%v; 2 s later the graph is fine, Get = %q err=%v\n", bug, err, v, err2)
	}
}
