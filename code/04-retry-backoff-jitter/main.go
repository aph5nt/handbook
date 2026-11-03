// Retry with backoff and jitter: five decoder pods retry against a price
// database that restarts at second 0 and is back at second 8.
//
// The random numbers come from the same small generator as the lesson
// (seed 11, multiply by 16807, mod 2147483647), so the retry times here are
// the dots in the animation. No sleeps: time is just added up.
package main

import (
	"fmt"
	"math"
)

const (
	pods = 5
	end  = 24.0 // seconds shown
)

const capSec = 16.0 // no wait is longer than this

var seed int64 = 11

func rnd() float64 {
	seed = seed * 16807 % 2147483647
	return float64(seed) / 2147483647
}

// wait is the delay before retry number n (0-based).
func wait(kind string, n int) float64 {
	if kind == "fixed" {
		return 1 // always one second
	}
	w := math.Min(capSec, math.Pow(2, float64(n))) // 1, 2, 4, 8, 16, 16 ...
	if kind == "jitter" {
		w = math.Max(0.3, rnd()*w) // full jitter: random in 0..w, floor 0.3 s
	}
	return w
}

// retries returns one pod's retry times up to the end of the window.
func retries(kind string) []float64 {
	var out []float64
	t := 0.0
	limit := 8 // attempts
	if kind == "fixed" {
		limit = 30
	}
	for n := 0; n < limit; n++ {
		t += wait(kind, n)
		if t > end {
			break
		}
		out = append(out, math.Round(t*10)/10)
	}
	return out
}

func main() {
	for _, kind := range []string{"fixed", "exponential", "jitter"} {
		seed = 11
		perSecond := make([]int, int(end)+1)
		total := 0
		for p := 0; p < pods; p++ {
			times := retries(kind)
			if p == 0 {
				fmt.Printf("%-11s pod 1 retries at %v\n", kind, times)
			}
			total += len(times)
			for _, t := range times {
				perSecond[int(t)]++
			}
		}
		peak, at := 0, 0
		for s := 8; s <= int(end); s++ { // after the database is back
			if perSecond[s] > peak {
				peak, at = perSecond[s], s
			}
		}
		fmt.Printf("%-11s %d retries in all, after second 8 the peak is %d retries in second %d\n", kind, total, peak, at)
	}
}
