// Three ways to count calls. Replays the lesson 13-rate-limit-algorithms:
// a token bucket, a fixed window and its burst at the boundary, a counter
// per gateway copy, and the fix with one shared counter.
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
)

// Bucket holds at most capacity tokens. One token drips in every refill
// seconds. The balance is counted in seconds of drip, so it stays a whole number.
type Bucket struct {
	capacity, refill int
	credit, last     int
}

// Take spends a token at time now. It says how long to wait when there is none.
func (b *Bucket) Take(now int) (bool, int) {
	b.credit = min(b.capacity*b.refill, b.credit+now-b.last)
	b.last = now
	if b.credit >= b.refill {
		b.credit -= b.refill
		return true, 0
	}
	return false, b.refill - b.credit
}

// Gateway applies one bucket per caller.
type Gateway struct {
	mu      sync.Mutex
	now     int
	buckets map[string]*Bucket
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	caller := r.Header.Get("X-Caller")
	b, ok := g.buckets[caller]
	if !ok {
		b = &Bucket{capacity: 5, refill: 12, credit: 60, last: g.now}
		g.buckets[caller] = b
	}
	if ok, wait := b.Take(g.now); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(wait))
		w.WriteHeader(http.StatusTooManyRequests)
	}
}

// FixedWindow counts per calendar minute and resets at :00.
type FixedWindow struct {
	limit, window, count int
}

func (f *FixedWindow) Allow(second int) bool {
	if m := second / 60; m != f.window {
		f.window, f.count = m, 0
	}
	if f.count >= f.limit {
		return false
	}
	f.count++
	return true
}

// Sliding keeps the calls of the last 60 seconds.
type Sliding struct {
	limit int
	calls []int
}

func (s *Sliding) Allow(second int) bool {
	kept := s.calls[:0]
	for _, c := range s.calls {
		if second-c < 60 {
			kept = append(kept, c)
		}
	}
	s.calls = kept
	if len(s.calls) >= s.limit {
		return false
	}
	s.calls = append(s.calls, second)
	return true
}

func main() {
	g := &Gateway{buckets: map[string]*Bucket{}}
	srv := httptest.NewServer(g)
	defer srv.Close()
	send := func(caller string) (int, string) {
		req, _ := http.NewRequest("GET", srv.URL, nil)
		req.Header.Set("X-Caller", caller)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			panic(err)
		}
		res.Body.Close()
		return res.StatusCode, res.Header.Get("Retry-After")
	}

	fmt.Println("--- token bucket: capacity 5, one token every 12 s")
	for i := 1; i <= 3; i++ {
		code, _ := send("agent")
		fmt.Println("agent burst", i, code)
	}
	for t := 0; t < 5; t++ {
		g.now = t
		code, _ := send("script")
		fmt.Println("script at", t, "s:", code)
	}
	g.now = 5
	code, after := send("script")
	fmt.Println("script at 5 s:", code, "Retry-After", after)
	passed := []int{}
	for t := 6; t <= 60; t++ {
		g.now = t
		if code, _ := send("script"); code == 200 {
			passed = append(passed, t)
		}
	}
	fmt.Println("one query every second, passes at:", passed)

	fmt.Println("--- fixed window: 5 per calendar minute")
	f := &FixedWindow{limit: 5, window: -1}
	pass := 0
	for i := 0; i < 5; i++ {
		if f.Allow(59) { // 12:00:59
			pass++
		}
	}
	for i := 0; i < 5; i++ {
		if f.Allow(60) { // 12:01:00
			pass++
		}
	}
	fmt.Println("five at 12:00:59, five at 12:01:00: passed", pass, "in two seconds")

	copies := make([]*FixedWindow, 4)
	for i := range copies {
		copies[i] = &FixedWindow{limit: 5, window: -1}
	}
	pass = 0
	for i := 0; i < 40; i++ {
		if copies[i%4].Allow(10) {
			pass++
		}
	}
	fmt.Println("a counter per gateway copy, 4 copies: passed", pass, "in one minute")

	fmt.Println("--- the fix: a sliding window in one shared store")
	shared := &Sliding{limit: 5}
	pass = 0
	for i := 0; i < 5; i++ {
		if shared.Allow(59) {
			pass++
		}
	}
	for i := 0; i < 5; i++ {
		if shared.Allow(60) {
			pass++
		}
	}
	fmt.Println("the same boundary burst: passed", pass)
	shared = &Sliding{limit: 5}
	pass = 0
	for i := 0; i < 40; i++ { // all 4 copies ask the one store
		if shared.Allow(10) {
			pass++
		}
	}
	fmt.Println("all 4 copies on the one store: passed", pass)
}
