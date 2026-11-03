// Capacity planning: the story of the lesson 12-capacity-planning.html.
// A serving lane peaks at 120 query units a second. A launch should bring
// 2.5 times that. The team load tests to the knee, counts servers, adds a
// spare (N+1), and orders inside the lead time.
package main

import "fmt"

// Step is one rate of the load test on one server and the p99 it gave.
type Step struct {
	Rate   int // query units a second
	P99    int // milliseconds; 0 means the server crashed
	Broken bool
}

// Knee is the highest rate before p99 jumps by more than 1.5 times from one
// step to the next. Past it, a small spike makes a deep queue.
func Knee(test []Step) int {
	knee := test[0].Rate
	for i := 1; i < len(test); i++ {
		if test[i].Broken || test[i].P99*2 > test[i-1].P99*3 {
			return knee
		}
		knee = test[i].Rate
	}
	return knee
}

// Servers counts how many keep the peak under the knee with any one lost.
func Servers(peak, knee int) (n int, spare int) {
	n = (peak + knee - 1) / knee // round up
	for (n+spare-1)*knee < peak {
		spare++ // N+1: the peak must fit with one server down
	}
	return n, spare
}

// OrderBy is the last week the order can go out and still land before launch.
func OrderBy(launchWeek, leadWeeks int) int { return launchWeek - leadWeeks }

func main() {
	today := 120
	peak := today * 5 / 2
	fmt.Printf("today %d, launch forecast 2.5 x %d = %d units a second\n", today, today, peak)

	test := []Step{{10, 20, false}, {20, 21, false}, {30, 22, false}, {40, 23, false},
		{50, 25, false}, {60, 28, false}, {70, 31, false}, {80, 60, false},
		{90, 150, false}, {100, 0, true}}
	knee := Knee(test)
	fmt.Printf("load test, real mix: p99 31 ms at 70, 60 ms at 80 -> knee at %d per server\n", knee)

	n, spare := Servers(peak, knee)
	fmt.Printf("%d / %d rounds up to %d servers\n", peak, knee, n)
	fmt.Printf("one lost: %d x %d = %d < %d, so add %d spare -> %d servers (N+1)\n",
		n-1, knee, (n-1)*knee, peak, spare, n+spare)
	fmt.Printf("launch in week 6, lead time 4 weeks: order by week %d, so order now\n", OrderBy(6, 4))

	fmt.Println("== Planned on the average")
	crash := test[len(test)-1].Rate
	fmt.Printf("average 60, crash point %d: 2 servers look like %d\n", crash, 2*crash)
	fmt.Printf("real: 2 x %d = %d at the knee, launch brings %d: %d over\n", knee, 2*knee, peak, peak-2*knee)
	fmt.Printf("ordered on launch day (week 6): lands in week %d, after the peak\n", 6+4)
}
