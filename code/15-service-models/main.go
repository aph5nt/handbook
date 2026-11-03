// Who runs which layer in each service model, and why a long-running
// indexer does not fit a function: a time limit and no memory between calls.
package main

import "fmt"

var layers = []string{"code", "runtime", "operating system", "servers"}

// provider runs the layers from this index down.
var models = []struct {
	Name         string
	ProviderFrom int
}{
	{"on-prem", 4},
	{"IaaS", 3},
	{"managed Kubernetes", 2},
	{"PaaS", 1},
	{"FaaS", 1},
	{"SaaS", 0},
}

// runAsFunction replays a block range under a time limit. A function
// remembers nothing, so each call starts the range again.
func runAsFunction(rangeMinutes, limitMinutes, calls int, checkpointOutside bool) (done int) {
	for c := 1; c <= calls; c++ {
		progress := min(limitMinutes, rangeMinutes-done)
		if checkpointOutside {
			done += progress
		} else if progress < rangeMinutes {
			continue // timed out: all progress lost
		} else {
			done = rangeMinutes
		}
		if done >= rangeMinutes {
			return done
		}
	}
	return done
}

func main() {
	for _, m := range models {
		yours := layers[:m.ProviderFrom]
		fmt.Printf("%-19s you run %d layers: %v\n", m.Name, len(yours), yours)
	}
	const rangeMin, limitMin = 40, 15
	fmt.Printf("A block range of %d minutes, a function limit of %d minutes:\n", rangeMin, limitMin)
	fmt.Printf("  no memory, 100 calls: %d of %d minutes done\n", runAsFunction(rangeMin, limitMin, 100, false), rangeMin)
	fmt.Printf("  checkpoint in a database, 3 calls: %d of %d minutes done\n", runAsFunction(rangeMin, limitMin, 3, true), rangeMin)
}
