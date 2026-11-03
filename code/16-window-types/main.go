// Window types: trader 0xab.. makes seven swaps between 12:00 and 12:15.
// Times are minutes after 12:00. The same swaps go into tumbling, sliding
// and session windows.
package main

import (
	"fmt"
	"math"
	"sort"
)

var swaps = []float64{1, 2.5, 4, 6, 7, 12, 13.5}

// Tumbling: each swap lands in exactly one fixed window.
func Tumbling(times []float64, size float64) map[float64]int {
	count := map[float64]int{}
	for _, t := range times {
		start := math.Floor(t/size) * size // 6 -> 5: window 12:05
		count[start]++
	}
	return count
}

// Sliding: each swap lands in length/step overlapping windows.
func Sliding(times []float64, length, step float64) map[float64]int {
	count := map[float64]int{}
	for _, t := range times {
		last := math.Floor(t/step) * step // the newest window holding t
		for start := last; start > t-length; start -= step {
			if start >= 0 {
				count[start]++
			}
		}
	}
	return count
}

type Session struct {
	Start, Last float64
	Count       int
}

// Sessions groups one trader's swaps into bursts. A gap longer than gap
// closes the session. A session longer than maxLen is cut (0 = no cap).
func Sessions(times []float64, gap, maxLen float64) (closed []Session, open *Session) {
	for _, t := range times {
		switch {
		case open == nil:
			open = &Session{Start: t, Last: t}
		case t-open.Last > gap: // quiet gap: close and start a new one
			closed = append(closed, *open)
			open = &Session{Start: t, Last: t}
		case maxLen > 0 && t-open.Start > maxLen: // the cap
			closed = append(closed, *open)
			open = &Session{Start: t, Last: t}
		default:
			open.Last = t // extends the end
		}
		open.Count++
	}
	return closed, open
}

func show(name string, m map[float64]int) {
	keys := []float64{}
	for k := range m {
		keys = append(keys, k)
	}
	sort.Float64s(keys)
	total := 0
	fmt.Print(name, ":")
	for _, k := range keys {
		fmt.Printf(" 12:%02.0f=%d", k, m[k])
		total += m[k]
	}
	fmt.Printf("  (sum %d from %d swaps)\n", total, len(swaps))
}

func main() {
	show("tumbling 5 min", Tumbling(swaps, 5))
	sl := Sliding(swaps, 10, 5)
	fmt.Printf("sliding 10/5: 12:00 to 12:10=%d, 12:05 to 12:15=%d, windows per swap %v\n", sl[0], sl[5], 10/5)
	fmt.Printf("sliding 10/1: windows per swap %d, a 24 h window by 1 min: %d\n", 10/1, 24*60/1)
	closed, open := Sessions(swaps, 3, 0)
	for _, s := range closed {
		fmt.Printf("session closed: 12:%02.0f to 12:%02.0f, %d swaps, closes at 12:%02.0f (last + 3 min gap)\n", s.Start, s.Last, s.Count, s.Last+3)
	}
	fmt.Printf("session open: starts 12:%02.0f, %d swaps so far\n", open.Start, open.Count)
	bot := []float64{}
	for m := 0.0; m < 15; m++ {
		bot = append(bot, m)
	}
	closed, open = Sessions(bot, 3, 0)
	fmt.Printf("bot, no cap: %d closed, open session holds %d swaps and never closes\n", len(closed), open.Count)
	day := []float64{}
	for m := 0.0; m < 180; m++ {
		day = append(day, m)
	}
	closed, _ = Sessions(day, 3, 60)
	fmt.Printf("bot for 3 hours, cap 60 min: %d sessions closed and emitted\n", len(closed))
}
