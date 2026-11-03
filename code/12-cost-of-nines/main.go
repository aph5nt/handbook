// Replays the cost of nines: parts in series multiply availability, copies in
// parallel multiply the chance of failure, and a shared failure undoes it.
package main

import (
	"fmt"
	"math"
)

// Series: every part must be up.
func Series(parts ...float64) float64 {
	a := 1.0
	for _, p := range parts {
		a *= p
	}
	return a
}

// Parallel: one copy is enough, so all must be down together.
func Parallel(copies ...float64) float64 {
	down := 1.0
	for _, c := range copies {
		down *= 1 - c
	}
	return 1 - down
}

// Correlated: share is the part of the failures both copies have together.
func Correlated(a, share float64) float64 {
	shared := (1 - a) * share
	alone := (1 - a) * (1 - share)
	return 1 - (shared + alone*alone)
}

// DownHoursPerYear turns an availability into hours down a year.
func DownHoursPerYear(a float64) float64 {
	return 8760 * (1 - a)
}

// CostFactor: each nine costs about ten times more.
func CostFactor(fromNines, toNines int) float64 {
	return math.Pow(10, float64(toNines-fromNines))
}

// Pick judges a target against what the path gives and what users pay for.
func Pick(target, path, pays float64) string {
	if target > path {
		return "a lie: the path cannot deliver it"
	}
	if target > pays {
		return "waste: users do not pay for it"
	}
	return "ok"
}

func main() {
	fmt.Printf("proxy alone: %.1f%%, %.1f h down a year\n", 99.9, DownHoursPerYear(0.999))
	fmt.Printf("proxy + graph: %.2f%%\n", Series(0.999, 0.999)*100)
	path := Series(0.999, 0.999, 0.999)
	fmt.Printf("proxy + graph + warehouse: %.2f%%, %.1f h down a year\n", path*100, DownHoursPerYear(path))
	fmt.Println("asked 99.99 on this path:", Pick(0.9999, path, 0.9999))
	two := Parallel(0.99, 0.99)
	fmt.Printf("two independent 99%% providers: %.2f%%\n", two*100)
	fmt.Printf("all failures shared: %.2f%%, 90%% shared: %.2f%%\n", Correlated(0.99, 1)*100, Correlated(0.99, 0.9)*100)
	for _, a := range []float64{0.99, 0.999, 0.9999, 0.99999} {
		h := DownHoursPerYear(a)
		fmt.Printf("%.3f%%: %.1f h = %.0f min down a year\n", a*100, h, h*60)
	}
	fmt.Printf("three nines to four: about %.0f times the cost\n", CostFactor(3, 4))
	fmt.Println("free tier, 99.5 on the 99.7 path, users pay 99.5:", Pick(0.995, path, 0.995))
	fmt.Println("free tier, 99.99 on a 99.99 path, users pay 99.5:", Pick(0.9999, 0.9999, 0.995))
	fmt.Println("payment rail, 99.99 on two zones, users pay 99.99:", Pick(0.9999, two, 0.9999))
}
