// Windows and watermarks: swaps per minute by event time.
//
// A slice in arrival order stands in for the stream. The model is the one in
// handbook/code/16-watermarks: watermark = newest event time minus the out of
// order bound, a closed window keeps its state for the allowed lateness, and
// anything later goes to a side output. Times are seconds after 12:00:00. The
// 10 s bound and the 60 s lateness are this example's. Deterministic.
package main

import (
	"fmt"
	"sort"
)

const windowSize, outOfOrder = 60, 10 // seconds

// arrival order: 12:00:58 arrives after 12:01:05, 12:00:50 arrives very late.
var swaps = []int{5, 20, 38, 52, 65, 58, 95, 118, 50, 130, 165}

func clock(s int) string { return fmt.Sprintf("12:%02d:%02d", s/60, s%60) }

type counter struct {
	lateness, maxSeen int
	count             map[int]int
	closed            map[int]bool
	side              []int
}

func (c *counter) onSwap(t int) {
	if t > c.maxSeen {
		c.maxSeen = t
	}
	watermark := c.maxSeen - outOfOrder
	start := t - t%windowSize
	end := start + windowSize
	if watermark >= end+c.lateness {
		c.side = append(c.side, t) // too late: its state is gone
		return
	}
	c.count[start]++
	if c.closed[start] {
		fmt.Printf("  late %s: window %s updated to %d\n", clock(t), clock(start), c.count[start])
	}
	for s := range c.count {
		if !c.closed[s] && watermark >= s+windowSize {
			c.closed[s] = true // send c.count[s]
			fmt.Printf("  watermark %s: window %s closes with %d\n", clock(watermark), clock(s), c.count[s])
		}
	}
}

func replay(lateness int, extra ...int) *counter {
	c := &counter{lateness: lateness, count: map[int]int{}, closed: map[int]bool{}}
	for _, t := range append(append([]int{}, swaps...), extra...) {
		c.onSwap(t)
	}
	return c
}

// hopping lists the start of every window of the size that holds t.
func hopping(t, size, every int) []string {
	out := []string{}
	for s := t - t%every; s > t-size && s >= 0; s -= every {
		out = append(out, clock(s))
	}
	sort.Strings(out)
	return out
}

// sessions groups sorted event times: a gap wider than the rule splits them.
func sessions(ts []int, gap int) [][]int {
	sorted := append([]int{}, ts...)
	sort.Ints(sorted)
	out := [][]int{}
	for _, t := range sorted {
		if n := len(out); n > 0 && t-out[n-1][len(out[n-1])-1] <= gap {
			out[n-1] = append(out[n-1], t)
		} else {
			out = append(out, []int{t})
		}
	}
	return out
}

func main() {
	fmt.Println("== Tab 2: no allowed lateness ==")
	c := replay(0)
	fmt.Printf("12:00 count %d, too late (by default these are dropped): %v (in seconds)\n", c.count[0], c.side)

	fmt.Println("== Tab 2: 60 s allowed lateness, one more swap at 12:00:30 after the end ==")
	c = replay(60, 30)
	fmt.Printf("12:00 count %d, side output %v (in seconds)\n", c.count[0], c.side)

	fmt.Println("== Tab 3: other shapes ==")
	fmt.Println("hopping 2 minutes every minute, 12:01:05 is in:", hopping(65, 120, 60))
	fmt.Println("hopping 5 minutes every minute, 12:05:10 is in:", len(hopping(310, 300, 60)), "windows")
	fmt.Println("sessions, 15 s gap:", sessions([]int{5, 20, 38, 52}, 15))
	fmt.Println("late swap at 12:00:29:", sessions([]int{5, 20, 38, 52, 29}, 15))
}
