// Autoscaling and the cool-down: the story of the lesson 12-autoscaling.html,
// replayed. An autoscaler sizes a pool of indexer workers from queue depth per
// worker, waits out blips and cool-downs, and then fails on the wrong metric
// and with no cool-down.
package main

import (
	"fmt"
	"math"
)

const warmUp = 180 // seconds before a new worker passes its readiness check

// Scaler decides the pool size from one metric per worker.
type Scaler struct {
	Min, Max  int
	Target    float64 // wanted metric per worker
	Window    int     // seconds above target before scaling up
	CoolDown  int     // seconds after a change with no new decision
	DownAfter int     // seconds below target before scaling down
	OneByOne  bool    // scale down one worker at a time
	last      int     // time of the last change
	above     int     // since when above target, -1 if not
	below     int
}

func NewScaler(s Scaler) *Scaler {
	s.last, s.above, s.below = -1<<30, -1, -1
	return &s
}

// Desired is the proportional rule: workers * metric / target, rounded up.
func (s *Scaler) Desired(cur int, metric float64) int {
	d := int(math.Ceil(float64(cur) * metric / s.Target))
	return max(s.Min, min(s.Max, d))
}

// Tick runs every 30 s and returns the new pool size.
func (s *Scaler) Tick(now, cur int, metric float64) int {
	if now-s.last < s.CoolDown {
		return cur // cool-down: let the last change show in the metric
	}
	want := s.Desired(cur, metric)
	switch {
	case want > cur:
		s.below = -1
		if s.above < 0 {
			s.above = now
		}
		if now-s.above < s.Window {
			return cur // maybe a blip: wait
		}
		s.last, s.above = now, -1
		return want
	case want < cur:
		s.above = -1
		if s.below < 0 {
			s.below = now
		}
		if now-s.below < s.DownAfter {
			return cur
		}
		s.last, s.below = now, -1
		if s.OneByOne {
			return cur - 1 // keep spares in case the burst comes back
		}
		return want
	}
	s.above, s.below = -1, -1
	return cur
}

func good() *Scaler {
	return NewScaler(Scaler{Min: 2, Max: 8, Target: 20, Window: 60, CoolDown: 300, DownAfter: 600, OneByOne: true})
}

func main() {
	fmt.Println("== good: queue depth with a cool-down (target 20 per worker)")
	s, n := good(), 2
	// queue per worker, read every 30 s
	reading := func(t int) float64 {
		switch {
		case t < 30:
			return 10
		case t < 120:
			return 35
		case t < 390:
			return 30
		default:
			return 8
		}
	}
	for t := 0; t <= 990; t += 30 {
		m := reading(t)
		before := n
		n = s.Tick(t, n, m)
		switch {
		case t == 0:
			fmt.Printf("1 t=%ds queue %.0f per worker: desired %d (min 2), workers %d\n", t, m, s.Desired(before, m), n)
		case t == 60:
			fmt.Printf("2 t=%ds queue %.0f, above target for %ds of 60: wait\n", t, m, t-30)
		case n != before && n > before:
			fmt.Printf("3 t=%ds queue %.0f: desired ceil(%d*%.0f/20) = %d, add %d, ready at t=%ds\n", t, m, before, m, n, n-before, t+warmUp)
		case t == 120:
			fmt.Printf("4 t=%ds queue %.0f would want %d, but cool-down until t=%ds: workers %d\n", t, m, s.Desired(n, m), s.last+300, n)
		case t == 390:
			fmt.Printf("5 t=%ds queue %.0f: desired %d, below target from now\n", t, m, s.Desired(n, m))
		case n < before:
			fmt.Printf("6 t=%ds below for 600 s: remove 1, workers %d (min 2)\n", t, n)
		}
	}

	fmt.Println("== bad: wrong metric, no cool-down, spike")
	cpu := NewScaler(Scaler{Min: 2, Max: 8, Target: 70, Window: 60, CoolDown: 300, DownAfter: 600, OneByOne: true})
	n = 2
	for t, q := 0, 10.0; t <= 600; t, q = t+30, q+2.5 {
		n = cpu.Tick(t, n, 15) // CPU stays at 15 percent: workers wait on writes
		if t == 0 || t == 600 {
			fmt.Printf("1 t=%ds CPU 15%%, queue %.0f per worker: desired %d, workers %d\n", t, q, cpu.Desired(n, 15), n)
		}
	}
	flap := NewScaler(Scaler{Min: 2, Max: 8, Target: 20})
	n, sizes, starts := 2, []int{2}, 0
	for k, m := range []float64{60, 6, 60, 6, 60, 6, 60, 6} {
		before := n
		n = flap.Tick(k*30, n, m)
		if n > before {
			starts += n - before
		}
		sizes = append(sizes, n)
	}
	fmt.Println("2 no cool-down, readings 60, 6, 60, 6...: sizes", sizes)
	fmt.Printf("3 %d worker starts in 4 min, each %d s of warm-up\n", starts, warmUp)
	fix := good()
	n, sizes = 2, []int{2}
	for k, m := range []float64{60, 60, 60, 6, 60, 6, 60, 6} {
		n = fix.Tick(k*30, n, m)
		sizes = append(sizes, n)
	}
	fmt.Println("  with the fix (cool-down 300 s > warm-up 180 s):", sizes)
	s, n = good(), 2
	for t := 0; t <= 60; t += 30 {
		n = s.Tick(t, n, 100) // a spike: queue full
	}
	fmt.Printf("4 spike at t=0: scale to %d at t=60s, ready at t=%ds, queue full for %d min\n", n, 60+warmUp, (60+warmUp)/60)
}
