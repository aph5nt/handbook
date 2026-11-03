// Replays the game day: measure, write a hypothesis, kill one pod, watch
// freshness minute by minute, and abort at the agreed line.
package main

import "fmt"

const steady = 20 // blocks behind on a normal day

// Sync is how the graph sync behaves after a pod dies.
type Sync struct {
	Reconnects bool
}

// Behind is the freshness m minutes after the kill.
func (s Sync) Behind(m int) int {
	if !s.Reconnects {
		return 30 + 30*m // stuck: falls 30 further behind each minute
	}
	switch {
	case m <= 1:
		return 30 + 30*m // the pod is coming back
	case m == 2:
		return 40
	}
	return steady
}

// Experiment is the game day shape.
type Experiment struct {
	Hypothesis string
	AbortAt    int
	HasAbort   bool
	Pods, Of   int
}

// Blast is the share of the pods the fault may touch.
func (e Experiment) Blast() int {
	return 100 * e.Pods / e.Of
}

// Run watches each minute and presses the switch at the line.
func Run(e Experiment, s Sync, minutes int) (peak, at int, verdict string) {
	for m := 0; m < minutes; m++ {
		b := s.Behind(m)
		if b > peak {
			peak, at = b, m
		}
		if e.HasAbort && b > e.AbortAt {
			return peak, m, "hypothesis false: abort, record the gap"
		}
	}
	if !e.HasAbort {
		return peak, at, "no line to judge it by: an outage you caused"
	}
	return peak, at, "hypothesis holds"
}

// NextStep widens only what held.
func NextStep(verdict string, pods int) string {
	if verdict != "hypothesis holds" {
		return "fix the gap, run the same fault again"
	}
	return fmt.Sprintf("widen: %d pods, then production in working hours", pods+1)
}

func main() {
	e := Experiment{"if one DB pod dies, freshness stays under 100 blocks", 100, true, 1, 3}
	fmt.Println("steady state:", steady, "blocks. hypothesis:", e.Hypothesis)
	fmt.Println("blast radius:", e.Blast(), "percent of the pods, staging")
	peak, at, v := Run(e, Sync{false}, 30)
	fmt.Printf("first run, killed at 11:01: %d blocks at 11:%02d: %s\n", peak, 1+at, v)
	fmt.Println("restart the sync: back to", steady, "by 11:06. next:", NextStep(v, 1))
	peak, _, v = Run(e, Sync{true}, 30)
	fmt.Printf("after the fix: peak %d: %s. next: %s\n", peak, v, NextStep(v, 1))

	fmt.Println("\nreckless: production, no line, no switch")
	r := Sync{false}
	fmt.Println("after 4 minutes:", r.Behind(4), "blocks. is this normal? nobody knows")
	fmt.Println("after 24 minutes, a manual fix found:", r.Behind(24))
	fmt.Println("after 27 minutes, a second pod down:", r.Behind(27))
	_, _, v = Run(Experiment{HasAbort: false}, r, 28)
	fmt.Println(v)
}
