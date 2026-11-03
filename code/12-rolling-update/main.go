// Command rollingupdate replays the lesson: four copies of the proxy are replaced one
// at a time with max surge 1 and max unavailable 0, the way a Kubernetes Deployment
// rolls. A new copy takes traffic only after its readiness check passes. Then a version
// that is ready but wrong rolls out everywhere, and a gate between steps stops it.
package main

import "fmt"

// Copy is one running copy of the proxy.
type Copy struct {
	Version string
	Ready   bool    // passed its readiness check: it is in the balancer
	ErrRate float64 // share of queries that fail on this copy
}

// Release describes the new version.
type Release struct {
	Version     string
	StartsReady bool    // the readiness check passes
	ErrRate     float64 // what real traffic will show
}

// ErrorRate is the share of all queries that fail: the balancer spreads them evenly over ready copies.
func ErrorRate(copies []*Copy) float64 {
	n, sum := 0, 0.0
	for _, c := range copies {
		if c.Ready {
			n++
			sum += c.ErrRate
		}
	}
	return sum / float64(n)
}

// Roll replaces the copies one at a time. gate, when set, may stop the roll after a step.
// waitMin is how long Kubernetes waits for readiness before it marks the roll failed.
func Roll(copies []*Copy, r Release, gate func(newRate, oldRate float64) bool, waitMin int) []*Copy {
	old := 0.001
	for k := range copies {
		surge := &Copy{Version: r.Version, ErrRate: r.ErrRate} // max surge 1: a fifth copy starts
		if !r.StartsReady {
			fmt.Printf("  step %d: %s never ready after %d minutes: roll stalled, %d old copies serve\n", k+1, r.Version, waitMin, len(copies))
			return copies
		}
		surge.Ready = true // readiness passed: it joins the balancer
		copies[k] = surge  // only now does one old copy stop (max unavailable 0)
		fmt.Printf("  step %d: %d of %d on %s, errors %.1f%%\n", k+1, k+1, len(copies), r.Version, 100*ErrorRate(copies))
		if gate != nil && !gate(surge.ErrRate, old) {
			copies[k] = &Copy{Version: "v1", Ready: true, ErrRate: old}
			fmt.Printf("  gate: new copy %.1f%% vs old %.1f%%: undo, %d of %d on %s\n", 100*surge.ErrRate, 100*old, 0, len(copies), r.Version)
			return copies
		}
	}
	return copies
}

func fleet() []*Copy {
	c := make([]*Copy, 4)
	for k := range c {
		c[k] = &Copy{Version: "v1", Ready: true, ErrRate: 0.001}
	}
	return c
}

func main() {
	fmt.Println("good v2:")
	Roll(fleet(), Release{"v2", true, 0.001}, nil, 10)

	fmt.Println("v2 that cannot load its config:")
	Roll(fleet(), Release{"v2", false, 0}, nil, 10)

	fmt.Println("v2 ready but one query in ten fails, no gate:")
	Roll(fleet(), Release{"v2", true, 0.1}, nil, 10)

	fmt.Println("same v2, with a gate after each step:")
	gate := func(newRate, oldRate float64) bool { return newRate <= 2*oldRate }
	Roll(fleet(), Release{"v2", true, 0.1}, gate, 10)
}
