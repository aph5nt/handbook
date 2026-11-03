// Command ratelimitalgorithms replays the lesson: the same query times go through
// four limiters (token bucket, leaky bucket, fixed window, sliding window) with a
// limit of 5 a second, and the program prints which queries pass.
package main

import (
	"fmt"
	"math"
	"strings"
)

// Limit is the allowed rate: 5 queries a second.
const Limit = 5

// TokenBucket starts full with Limit tokens and refills Limit tokens a second.
func TokenBucket(ts []float64) []bool {
	tokens, last := float64(Limit), 0.0
	out := make([]bool, len(ts))
	for i, t := range ts {
		tokens = math.Min(Limit, tokens+(t-last)*Limit) // drip in since the last query
		last = t
		if tokens >= 1 {
			tokens--
			out[i] = true
		}
	}
	return out
}

// LeakyBucket queues up to Limit queries and sends one out every 1/Limit seconds.
// It returns, per query, the send time, or -1 when the queue was full.
func LeakyBucket(ts []float64) []float64 {
	var sends []float64
	lastSend := -1.0
	out := make([]float64, len(ts))
	for i, t := range ts {
		waiting := 0
		for _, s := range sends {
			if s > t {
				waiting++ // still in the queue at time t
			}
		}
		if waiting >= Limit {
			out[i] = -1 // queue full: refused
			continue
		}
		send := math.Max(t, lastSend+1.0/Limit) // one every 0.2 s
		lastSend = send
		sends = append(sends, send)
		out[i] = send
	}
	return out
}

// FixedWindow keeps one counter per clock second.
func FixedWindow(ts []float64) []bool {
	count := map[int]int{}
	out := make([]bool, len(ts))
	for i, t := range ts {
		w := int(math.Floor(t)) // window 0 is 0 to 1 s, window 1 is 1 to 2 s
		if count[w] < Limit {
			count[w]++
			out[i] = true
		}
	}
	return out
}

// SlidingWindow looks back exactly one second from each query and counts what passed.
func SlidingWindow(ts []float64) []bool {
	var passed []float64
	out := make([]bool, len(ts))
	for i, t := range ts {
		n := 0
		for _, p := range passed {
			if p > t-1 {
				n++
			}
		}
		if n < Limit {
			passed = append(passed, t)
			out[i] = true
		}
	}
	return out
}

// SlidingCounter keeps two counters per key: this second and the last one.
// The last one counts in proportion to how much of it the one-second look-back still covers.
func SlidingCounter(ts []float64) []bool {
	count := map[int]float64{}
	out := make([]bool, len(ts))
	for i, t := range ts {
		w := int(math.Floor(t))
		overlap := 1 - (t - float64(w)) // share of the last window still inside the look-back
		estimate := count[w-1]*overlap + count[w]
		if estimate+1 <= Limit {
			count[w]++
			out[i] = true
		}
	}
	return out
}

func show(name string, ok []bool) {
	var b strings.Builder
	pass := 0
	for _, v := range ok {
		if v {
			b.WriteString("+")
			pass++
		} else {
			b.WriteString("x")
		}
	}
	fmt.Printf("  %-15s %s  %d passed, %d refused\n", name, b.String(), pass, len(ok)-pass)
}

func showLeaky(ts []float64) {
	sends := LeakyBucket(ts)
	var b strings.Builder
	pass, late := 0, 0
	for i, s := range sends {
		switch {
		case s < 0:
			b.WriteString("x")
		case s-ts[i] > 0.05:
			b.WriteString("~") // sent later, from the queue
			pass++
			late++
		default:
			b.WriteString("+")
			pass++
		}
	}
	fmt.Printf("  %-15s %s  %d passed (%d delayed), %d refused\n", "leaky bucket", b.String(), pass, late, len(sends)-pass)
	for i, s := range sends {
		if s >= 0 && s-ts[i] > 0.05 {
			fmt.Printf("    query at %.2f s sent at %.2f s\n", ts[i], s)
		}
	}
}

func run(title string, ts []float64) {
	fmt.Println(title)
	show("token bucket", TokenBucket(ts))
	showLeaky(ts)
	show("fixed window", FixedWindow(ts))
	show("sliding window", SlidingWindow(ts))
}

func main() {
	// Tab 1: a burst of 8 in under half a second, 3 around 1.5 s, 2 near 2.5 s.
	burst := []float64{.1, .15, .2, .25, .3, .35, .4, .45, 1.5, 1.6, 1.7, 2.5, 2.6}
	run("Same burst, four limiters (+ pass, ~ delayed, x refused):", burst)

	// Tab 2: 5 just before 1 s, 5 from 1 s on, 1 at 2.5 s.
	edge := []float64{.75, .8, .85, .9, .95, 1, 1.05, 1.1, 1.15, 1.2, 2.5}
	fmt.Println()
	run("The window edge:", edge)

	// The cheap fix: two counters per key instead of one timestamp per query.
	show("two counters", SlidingCounter(edge))
	fmt.Printf("  at 1.05 s: estimate = 0 + 5 x %.2f = %.2f, one more makes %.2f > %d: refuse\n", 0.95, 5*0.95, 5*0.95+1, Limit)
}
