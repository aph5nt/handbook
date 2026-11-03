// Replays the SLO lesson: 43 minutes of error budget, spent event by event,
// and the release gate that reads what is left.
package main

import "fmt"

const minutesIn30Days = 30 * 24 * 60
const behindLimit = 100 // blocks behind the chain head

// Good is the SLI check for one minute: is the data fresh enough?
func Good(blocksBehind int) bool {
	return blocksBehind < behindLimit
}

// Budget is how many bad minutes the SLO allows in 30 days.
func Budget(slo float64) int {
	return int(minutesIn30Days * (1 - slo))
}

// CountBad reads one lag sample per minute and counts the misses.
func CountBad(lagPerMinute []int) int {
	bad := 0
	for _, behind := range lagPerMinute {
		if !Good(behind) {
			bad++
		}
	}
	return bad
}

// Gate is the rule agreed before the bad week.
func Gate(left, budget int) string {
	if left <= 0 {
		return "releases stopped"
	}
	if left*2 < budget {
		return "slow down"
	}
	return "ship"
}

// Review refuses the targets the second tab shows.
func Review(slo, sla, dependency float64) []string {
	var faults []string
	if slo >= 1 {
		faults = append(faults, "budget is zero: every release is a breach")
	}
	if sla >= slo {
		faults = append(faults, "the contract breaks before the alarm fires")
	}
	if slo > dependency {
		faults = append(faults, "promise above the dependency")
	}
	return faults
}

// DownPerYear turns an availability into minutes down a year.
func DownPerYear(avail float64) float64 {
	return 365 * 24 * 60 * (1 - avail)
}

// lag builds n minutes of samples: bad of them at 150 blocks, the rest at 20.
func lag(n, bad int) []int {
	s := make([]int, n)
	for k := range s {
		s[k] = 20
		if k < bad {
			s[k] = 150
		}
	}
	return s
}

func main() {
	budget := Budget(0.999)
	fmt.Printf("step 2: %d minutes x 0.001 = %d minutes of budget\n", minutesIn30Days, budget)
	left := budget
	events := []struct {
		day, bad int
		what     string
	}{{3, 2, "release"}, {8, 5, "node blip"}, {12, 22, "bad release"}, {15, 14, "release"}}
	for _, e := range events {
		left -= CountBad(lag(1440, e.bad))
		fmt.Printf("day %2d: %-11s costs %2d, left %2d, gate: %s\n", e.day, e.what, e.bad, left, Gate(left, budget))
	}
	fmt.Printf("SLA 99.5%% allows %d minutes, %d used: contract met\n", Budget(0.995), budget-left)

	fmt.Println("\ntab 2")
	fmt.Println("SLO 100%:", Budget(1.0), "minutes:", Review(1.0, 0.995, 1.0)[0])
	fmt.Println("SLA 99.95 on SLO 99.9:", Review(0.999, 0.9995, 1.0))
	serverBad, userBad := 4, CountBad(lag(minutesIn30Days, 120))
	fmt.Printf("server view: %d bad minutes = %.2f%%, user view: %d bad minutes = %.2f%%\n",
		serverBad, 100-float64(serverBad)/minutesIn30Days*100, userBad, 100-float64(userBad)/minutesIn30Days*100)
	fmt.Println("SLO 99.9 on a 99.5 node:", Review(0.999, 0.995, 0.995))
	for _, a := range []float64{0.99, 0.999, 0.9999, 0.99999} {
		fmt.Printf("%.3f%%: %.0f minutes down a year\n", a*100, DownPerYear(a))
	}
}
