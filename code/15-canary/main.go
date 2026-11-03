// Canary analysis for the query API: v2 earns traffic in steps, and each
// step compares v2's error rate with v1's over the same ten minutes.
package main

import (
	"fmt"
	"time"
)

// Window holds one version's counts for the last ten minutes.
type Window struct {
	Requests int
	Errors   int
}

func (w Window) Rate() float64 { return float64(w.Errors) / float64(w.Requests) }

// Metrics answers "what did this version do in the last ten minutes?".
// In production this is a Prometheus query with a version label.
type Metrics func(version string, weight int) Window

const (
	maxRatio    = 1.5  // v2 fails above 1.5 times v1's error rate
	minRequests = 1000 // fewer v2 requests cannot be judged
)

var steps = []int{5, 25, 50, 100}

// sleep is the ten-minute wait. A real controller sleeps; this replay does not.
var sleep = func(time.Duration) {}

// Rollout walks the steps and returns the outcome.
func Rollout(m Metrics, setWeight func(int)) string {
	for _, weight := range steps {
		setWeight(weight)
		sleep(10 * time.Minute) // let both versions serve for ten minutes
		base := m("v1", weight) // query with version="v1"
		can := m("v2", weight)  // query with version="v2"
		if can.Requests < minRequests {
			return "paused: not enough v2 traffic to judge"
		}
		fmt.Printf("  %3d%%  v1 %.2f%%  v2 %.2f%%  limit %.2f%%\n",
			weight, 100*base.Rate(), 100*can.Rate(), 100*base.Rate()*maxRatio)
		if can.Rate() > base.Rate()*maxRatio {
			setWeight(0) // roll back: all traffic to v1
			return fmt.Sprintf("aborted at %d%%", weight)
		}
	}
	return "promoted: v1 pods removed, v1 version kept for rollback"
}

// traffic: 100,000 requests in ten minutes, split by weight.
// v1 fails 0.5 percent of requests, v2 fails v2Rate.
func split(v2Rate float64) Metrics {
	return func(version string, weight int) Window {
		n := 100000 * weight / 100
		rate := v2Rate
		if version == "v1" {
			n = 100000 - n
			rate = 0.005
		}
		if n == 0 { // v1 has no pods at 100 percent, use its last window
			n, rate = 100000, 0.005
		}
		return Window{Requests: n, Errors: int(float64(n) * rate)}
	}
}

// blind is the failure: one overall error rate, no version label.
// Both "versions" read the same total, so v2 always looks like v1.
func blind(v2Rate float64) Metrics {
	return func(version string, weight int) Window {
		all := 100000
		v2 := all * weight / 100
		errs := int(float64(all-v2)*0.005 + float64(v2)*v2Rate)
		if version == "v1" {
			return Window{Requests: all, Errors: int(float64(all) * 0.005)} // last week's baseline
		}
		return Window{Requests: all, Errors: errs}
	}
}

func main() {
	set := func(w int) {}
	fmt.Println("A good v2 (0.5% errors):")
	fmt.Println(" ", Rollout(split(0.005), set))
	fmt.Println("A bad v2 (1.0% errors), metrics split by version:")
	fmt.Println(" ", Rollout(split(0.010), set))
	fmt.Println("The same bad v2, one overall error rate:")
	fmt.Println(" ", Rollout(blind(0.010), set))
}
