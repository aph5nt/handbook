// Command budget replays the error budget lesson: spend, a fast burn that
// pages, a slow burn that opens a ticket, a freeze, the window sliding, and a
// target of 100%.
package main

import (
	"fmt"
	"math"
)

const windowMin = 30 * 24 * 60.0 // 43,200 minutes in 30 days

type Spend struct {
	Name string
	Min  float64
}

type SLO struct {
	Target float64
	Spent  []Spend
}

func (s *SLO) Budget() float64 { return (1 - s.Target) * windowMin }

func (s *SLO) Spend(name string, errorRate, minutes float64) {
	s.Spent = append(s.Spent, Spend{name, errorRate * minutes})
}

func (s *SLO) Left() float64 {
	left := s.Budget()
	for _, x := range s.Spent {
		left -= x.Min
	}
	return left
}

func (s *SLO) Burn(errorRate float64) float64 {
	return errorRate / (1 - s.Target)
}

// Check picks the alert. The caller measures errorRate over the window.
func (s *SLO) Check(errorRate, windowHours float64) string {
	b := s.Burn(errorRate)
	if b >= 14.4 && windowHours >= 1 {
		return "page"
	}
	if b >= 1 && windowHours >= 72 {
		return "ticket"
	}
	return "nothing"
}

func (s *SLO) CanRelease() bool {
	if s.Left() <= 0 {
		return false
	}
	return true
}

// Slide drops spend that is now older than 30 days.
func (s *SLO) Slide(names ...string) {
	for _, n := range names {
		for i, x := range s.Spent {
			if x.Name == n {
				s.Spent = append(s.Spent[:i], s.Spent[i+1:]...)
				break
			}
		}
	}
}

func main() {
	s := &SLO{Target: 0.999}
	show := func(step string) {
		l := math.Max(s.Left(), 0) // 0.04 over budget shows as 0
		fmt.Printf("%s: %.1f min left (%.0f%%), can release: %v\n", step, l, l/s.Budget()*100, s.CanRelease())
	}
	fmt.Printf("step 2: budget %.1f min\n", s.Budget())
	s.Spend("release 7.2", 1, 9)
	show("step 3: 100% errors for 9 min")
	fmt.Printf("step 4: 2%% errors: burn %.0f, %s, full budget gone in %.0f hours\n", s.Burn(0.02), s.Check(0.02, 1), s.Budget()/0.02/60)
	s.Spend("fast burn", 0.02, 180)
	show("step 4: fixed after 3 hours")
	fmt.Printf("step 5: 0.2%% errors: burn %.0f, after 1 h: %s, after 3 days: %s, full budget gone in %.0f days\n", s.Burn(0.002), s.Check(0.002, 1), s.Check(0.002, 72), s.Budget()/0.002/60/24)
	s.Spend("slow burn", 0.002, 3*24*60)
	show("step 5: 3 days later")
	s.Spend("release 7.5", 1, 22)
	show("step 6: 22 min outage")
	s.Slide("release 7.2", "fast burn")
	show("step 7: old spend slides out")

	z := &SLO{Target: 1}
	fmt.Printf("bad: target 100%%: budget %.0f min\n", z.Budget())
	z.Spend("release 7.3", 1, 0.5)
	fmt.Printf("bad: 30 s of errors: %.1f min left, can release: %v\n", z.Left(), z.CanRelease())
}
