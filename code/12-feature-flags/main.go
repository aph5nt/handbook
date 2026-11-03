// Command featureflags replays the lesson: a new planner ships switched off, the flag
// ramps by a stable hash of the agent's API key, the error rate is compared per path,
// and a kill switch turns it off. Then the rot: flags nobody deleted, combinations nobody
// tested, a switch nothing reads, and the expiry check that stops it.
package main

import (
	"fmt"
	"hash/fnv"
	"math"
)

// Flag turns a code path on for a percentage of agents.
type Flag struct {
	Name    string
	Percent uint32 // 0 to 100
	Owner   string
	Expiry  string // YYYY-MM-DD
}

// Enabled decides by a hash of the key, so one agent always gets the same answer.
func (f *Flag) Enabled(apiKey string) bool {
	h := fnv.New32a()
	h.Write([]byte(f.Name + ":" + apiKey))
	return h.Sum32()%100 < f.Percent
}

// Traffic counts how many of n agents take the new path.
func Traffic(f *Flag, n int) int {
	on := 0
	for k := 0; k < n; k++ {
		if f.Enabled(fmt.Sprintf("agent-%05d", k)) {
			on++
		}
	}
	return on
}

// Expired lists the flags past their date: the check that fails the build.
func Expired(flags []Flag, today string) []string {
	var out []string
	for _, f := range flags {
		if f.Expiry < today { // YYYY-MM-DD sorts as text
			out = append(out, f.Name+" (owner "+f.Owner+")")
		}
	}
	return out
}

func main() {
	const agents = 10000
	f := &Flag{"new_walk_planner", 0, "graph team", "2026-12-01"}

	fmt.Printf("deploy, flag off: %d of %d agents on the new planner\n", Traffic(f, agents), agents)

	f.Percent = 10
	fmt.Printf("10%%: %d agents on the new planner\n", Traffic(f, agents))
	stable := f.Enabled("agent-00042")
	same := true
	for k := 0; k < 1000; k++ {
		same = same && f.Enabled("agent-00042") == stable
	}
	fmt.Printf("agent-00042 asked 1,000 times: always the same answer: %v\n", same)

	f.Percent = 50
	fmt.Printf("50%%: %d agents on the new planner, new path errors 4%%, old path 0.1%%\n", Traffic(f, agents))

	f.Percent = 0 // the kill switch
	fmt.Printf("kill switch: %d agents on the new planner, no build, no deploy\n", Traffic(f, agents))

	f.Percent = 100
	fmt.Printf("fixed, 100%%: %d agents, next step: delete the flag and the old planner\n", Traffic(f, agents))

	// The rot.
	fmt.Printf("\ncombinations: 3 flags %.0f, 10 flags %.0f, 200 flags about %.1e\n",
		math.Pow(2, 3), math.Pow(2, 10), math.Pow(2, 200))

	flags := []Flag{
		{"new_walk_planner", 100, "graph team", "2026-12-01"},
		{"kill_deep_walks", 0, "nobody", "2025-11-30"},
		{"old_cache_key", 100, "nobody", "2026-03-15"},
	}
	fmt.Println("build check on 2026-11-02, expired:", Expired(flags, "2026-11-02"))
}
