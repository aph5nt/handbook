// Rate limiting on a toy clock. A token bucket per API key keeps one buggy
// agent from hurting an analyst, a cost limit refuses one huge query, and
// a limit per IP address hits the wrong people.
package main

import "fmt"

type bucket struct {
	tokens, burst, refillPerSecond float64
	last                           float64
}

// allow refills by elapsed time, up to the burst, then takes one token.
func (b *bucket) allow(now float64) bool {
	b.tokens += (now - b.last) * b.refillPerSecond
	if b.tokens > b.burst {
		b.tokens = b.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

type limiter struct{ buckets map[string]*bucket }

// handle gives each key its own bucket, so one key cannot spend another's.
func (l *limiter) handle(key string, now float64) int {
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: 20, burst: 20, refillPerSecond: 10, last: now}
		l.buckets[key] = b
	}
	if !b.allow(now) {
		return 429 // with a Retry-After header
	}
	return 200
}

// costAllowed estimates a query before it runs. Counting calls cannot see
// this: it is one call that would scan the whole chain.
func costAllowed(estimatedRows, limit int) bool { return estimatedRows <= limit }

func main() {
	l := &limiter{map[string]*bucket{}}
	passed, refused := 0, 0
	for i := 0; i < 500; i++ {
		if l.handle("buggy-agent", float64(i)/500) == 200 {
			passed++
		} else {
			refused++
		}
	}
	fmt.Println("  buggy agent, 500 calls in 1 s ->", passed, "pass,", refused, "get 429")
	fmt.Println("  analyst, 2 calls in that second ->", l.handle("analyst", 0.2), l.handle("analyst", 0.7))
	fmt.Println("  whole-chain query, 2,000,000,000 rows, limit 1,000,000 ->", costAllowed(2_000_000_000, 1_000_000))

	byIP := &limiter{map[string]*bucket{}}
	blocked := 0
	for person := 0; person < 200; person++ {
		if byIP.handle("office-ip", 0) == 429 {
			blocked++
		}
	}
	fmt.Println("  by IP: office of 200 behind one address ->", blocked, "people blocked")
	hits := 0
	for addr := 0; addr < 1000; addr++ {
		if byIP.handle(fmt.Sprint("attacker-", addr), 0) == 200 {
			hits++
		}
	}
	fmt.Println("  by IP: attacker on 1,000 addresses ->", hits, "calls pass")
}
