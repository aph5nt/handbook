// Command canaryandrollback replays the lesson. Blue-green: one pointer moves all
// traffic to green and back. Canary: one server of ten gets the new version, its error
// rate is compared with the old servers in the same window, and the ramp goes on or
// rolls back on that comparison. Every number is computed from fixed counts.
package main

import "fmt"

// Fleet is one blue-green fleet.
type Fleet struct {
	Name    string
	Version string
	Column  string // the column this version reads from the shared database
}

// Router sends all traffic to the live fleet.
type Router struct{ Live *Fleet }

// Flip moves all traffic to the other fleet in one step.
func (r *Router) Flip(to *Fleet) { r.Live = to }

// Serve says whether the live fleet can read the shared table.
func (r *Router) Serve(dbColumns map[string]bool) string {
	if !dbColumns[r.Live.Column] {
		return r.Live.Name + " " + r.Live.Version + ": fails, column " + r.Live.Column + " is gone"
	}
	return r.Live.Name + " " + r.Live.Version + ": ok"
}

// Window is the requests and errors one group of servers saw in one watch window.
type Window struct{ Requests, Errors int }

// Rate is the error rate in percent.
func (w Window) Rate() float64 { return 100 * float64(w.Errors) / float64(w.Requests) }

// Verdict compares the canary with the old servers in the same window.
// It fails when the canary has at least minErrors errors and more than twice the old rate.
func Verdict(canary, baseline Window, minErrors int) (bool, string) {
	if canary.Errors >= minErrors && canary.Rate() > 2*baseline.Rate() {
		return false, fmt.Sprintf("roll back: canary %.1f%% vs old %.1f%%", canary.Rate(), baseline.Rate())
	}
	return true, fmt.Sprintf("go on: canary %.1f%% vs old %.1f%%", canary.Rate(), baseline.Rate())
}

// window builds the counts for n servers at 1,000 requests a minute for 10 minutes.
func window(servers int, errPerTenThousand int) Window {
	req := servers * 1000 * 10
	return Window{req, req * errPerTenThousand / 10000}
}

func main() {
	// Blue-green.
	blue := &Fleet{"blue", "v1", "agent_id"}
	green := &Fleet{"green", "v2", "agent_id"}
	db := map[string]bool{"agent_id": true}
	r := &Router{Live: blue}
	fmt.Println("live:", r.Serve(db))
	r.Flip(green)
	fmt.Println("flip, live:", r.Serve(db), "- 10 minutes later errors 4%")
	r.Flip(blue)
	fmt.Println("roll back, live:", r.Serve(db))

	// Replay, but v2 renamed the column in the shared database.
	green.Column = "agent_key"
	db = map[string]bool{"agent_key": true}
	r.Flip(green)
	r.Flip(blue)
	fmt.Println("rename replay, roll back, live:", r.Serve(db))

	// Canary: ten servers, the error rate in errors per 10,000 requests.
	fmt.Println()
	steps := []int{1, 5, 10}
	for _, n := range steps {
		if n == 10 {
			fmt.Println("10 of 10 on v2: done, 100% of traffic")
			break
		}
		ok, why := Verdict(window(n, 10), window(10-n, 10), 20)
		fmt.Printf("%d of 10 on v2 (%d%% of traffic): %s\n", n, n*10, why)
		if !ok {
			return
		}
	}
	ok, why := Verdict(window(1, 400), window(9, 10), 20)
	fmt.Printf("1 of 10 on v3: %d errors in %d requests, %s\n", window(1, 400).Errors, window(1, 400).Requests, why)
	if !ok {
		fmt.Println("canary drained and put back on v2: 9 servers never ran v3")
	}
}
