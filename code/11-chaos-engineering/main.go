// Command chaos replays the chaos engineering lesson: a steady-state metric,
// a hypothesis, control and experiment groups, two injected failures, the
// automatic stop line, and the re-run after the fix.
package main

import "fmt"

type Inject struct {
	KillCopy       bool // 1 of 10 Playback copies is dead
	ProfileExtraMs int
}

func Group(user int) string {
	switch user % 100 {
	case 0:
		return "control"
	case 1:
		return "experiment"
	}
	return "untouched"
}

// PlayStart is one attempt; k picks the copy and the Profile latency.
func PlayStart(k int, in Inject, profileTimeout bool) bool {
	t := 500
	if in.KillCopy && k%10 == 0 {
		t += 50
	}
	p := 150 + (k%11)*20 + in.ProfileExtraMs
	if profileTimeout && p > 300 {
		p = 300
	}
	return t+p <= 1000
}

// SPS counts successful starts out of 100 attempts in one second.
func SPS(in Inject, profileTimeout bool) int {
	n := 0
	for k := 0; k < 100; k++ {
		if PlayStart(k, in, profileTimeout) {
			n++
		}
	}
	return n
}

func Check(control, experiment int) string {
	if float64(experiment) < float64(control)*0.95 {
		return "STOP: fail"
	}
	return "pass"
}

func main() {
	fmt.Println("step 1: steady state, no injection")
	base := SPS(Inject{}, false)
	fmt.Printf("  play starts per second: %d of 100 attempts\n", base)
	fmt.Println("step 2: hypothesis: experiment within 5 % of control, stop line", float64(base)*0.95)
	fmt.Printf("step 3: user 4200 -> %s, user 4201 -> %s, user 4202 -> %s\n", Group(4200), Group(4201), Group(4202))
	fmt.Println("step 4: kill 1 of 10 Playback copies in the experiment group")
	exp := SPS(Inject{KillCopy: true}, false)
	fmt.Printf("step 5: control %d, experiment %d -> %s\n", base, exp, Check(base, exp))
	fmt.Println("step 6: +300 ms on Profile, Playback has no timeout on Profile")
	exp = SPS(Inject{ProfileExtraMs: 300}, false)
	fmt.Printf("  control %d, experiment %d, %d < %.0f -> %s\n", base, exp, exp, float64(base)*0.95, Check(base, exp))
	fmt.Println("step 7: fix: 300 ms timeout on Profile with a default profile, run again")
	exp = SPS(Inject{ProfileExtraMs: 300}, true)
	fmt.Printf("  control %d, experiment %d -> %s\n", base, exp, Check(base, exp))
}
