// Command vclock replays the vector clock lesson: per-node counters, the
// includes check, concurrent versions kept as siblings, the merge, and
// last-write-wins losing a write, with and without clock skew.
package main

import (
	"fmt"
	"time"
)

type Clock map[string]int

func Write(base Clock, c string) Clock {
	out := Clock{}
	for k, v := range base {
		out[k] = v
	}
	out[c]++
	return out
}

func Includes(a, b Clock) bool {
	for n, v := range b {
		if a[n] < v {
			return false
		}
	}
	return true
}

func Compare(a, b Clock) string {
	switch {
	case Includes(a, b) && Includes(b, a):
		return "same"
	case Includes(a, b):
		return "first is newer, drop second"
	case Includes(b, a):
		return "second is newer, drop first"
	}
	return "concurrent: keep both as siblings"
}

// MergeItems is the app rule for values: a union, so a removed item returns.
func MergeItems(a, b []string) (out []string) {
	seen := map[string]bool{}
	for _, x := range append(append([]string{}, a...), b...) {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func Merge(a, b Clock) Clock {
	// The clock merge is safe. A union of values is not: deleted items return.
	out := Write(a, "")
	delete(out, "")
	for n, v := range b {
		if v > out[n] {
			out[n] = v
		}
	}
	return out
}

func Truncate(c Clock, oldest string) Clock {
	if len(c) > 10 {
		delete(c, oldest)
	}
	return c
}

func LWW(t1, t2 time.Time) string {
	// t1 and t2 come from two machines' clocks, and those clocks drift.
	if t2.After(t1) {
		return "second wins, first dropped silently"
	}
	return "first wins, second dropped silently"
}

func main() {
	fmt.Println("vc tab")
	book := Write(Clock{}, "B")
	fmt.Println("step 1: book", book)
	phone := Write(book, "B")
	fmt.Println("step 2: phone through B", phone)
	laptop := Write(book, "C")
	fmt.Println("step 3: laptop through C", laptop)
	fmt.Printf("step 4: phone includes laptop? %v (C: %d < %d); laptop includes phone? %v (B: %d < %d)\n",
		Includes(phone, laptop), phone["C"], laptop["C"], Includes(laptop, phone), laptop["B"], phone["B"])
	fmt.Println("step 5:", Compare(phone, laptop))
	merged := Merge(phone, laptop)
	fmt.Println("step 6: merged", merged, "against phone:", Compare(merged, phone), "; against laptop:", Compare(merged, laptop))
	big := Clock{}
	for i := 0; i < 11; i++ {
		big[fmt.Sprintf("n%d", i)] = 1
	}
	child := Write(big, "n0")
	fmt.Println("step 7: real ancestor:", Compare(child, big))
	Truncate(child, "n5")
	fmt.Println("  after the child is cut at 10 entries:", Compare(child, big))

	fmt.Println("lww tab")
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	fmt.Println("step 1-3: phone 10:00:05, laptop 10:00:07 ->", LWW(t0.Add(5*time.Second), t0.Add(7*time.Second)))
	fmt.Println("step 4: phone really 10:00:06, laptop really 10:00:05, C clock +3 s stamps 10:00:08 ->", LWW(t0.Add(6*time.Second), t0.Add(8*time.Second)))
}
