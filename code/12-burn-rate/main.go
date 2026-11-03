// Replays the burn-rate lesson minute by minute over 72 hours: the fixed alert
// on one error series, the two-window alerts on the other.
package main

import "fmt"

const allowed = 0.1 // percent of errors the 99.9 SLO allows

// Burn is the average error rate of the last window minutes over the allowed rate.
func Burn(errs []float64, m, window int) float64 {
	from := m - window + 1
	if from < 0 {
		from = 0
	}
	sum := 0.0
	for _, e := range errs[from : m+1] {
		sum += e
	}
	return sum / float64(m+1-from) / allowed
}

// Fixed is the old alert: a single number, over one line.
func Fixed(errs []float64, m int) bool {
	return errs[m] > 1
}

// Fast pages: the long window proves it matters, the short one that it still runs.
func Fast(errs []float64, m int) bool {
	return Burn(errs, m, 60) > 14 && Burn(errs, m, 5) > 14
}

// Slow opens a ticket.
func Slow(errs []float64, m int) bool {
	return Burn(errs, m, 3*1440) > 1 && Burn(errs, m, 360) > 1
}

// BudgetLeft is the share of the 30-day budget still unspent after minute m.
func BudgetLeft(errs []float64, m int) float64 {
	spent := 0.0
	for _, e := range errs[:m+1] {
		spent += e / 100
	}
	return 1 - spent/(30*24*60*allowed/100)
}

// Edges reports the minute an alert starts and the minute it stops.
func Edges(errs []float64, alert func([]float64, int) bool) [][2]int {
	var out [][2]int
	on := false
	for m := range errs {
		now := alert(errs, m)
		if now && !on {
			out = append(out, [2]int{m, -1})
		}
		if !now && on {
			out[len(out)-1][1] = m
		}
		on = now
	}
	return out
}

func series(fill func(hour float64) float64) []float64 {
	e := make([]float64, 72*60)
	for m := range e {
		e[m] = fill(float64(m) / 60)
	}
	return e
}

func blip(h, at float64) bool { return h >= at && h < at+2.0/60 }

func at(h float64) string { return fmt.Sprintf("hour %.1f", h) }

func main() {
	static := series(func(h float64) float64 {
		switch {
		case blip(h, 10), blip(h, 20):
			return 2
		case h >= 30:
			return 0.3
		}
		return 0.05
	})
	fmt.Println("tab 1: fixed alert, errors above 1 percent")
	for _, e := range Edges(static, Fixed) {
		fmt.Printf("  PAGE at %s\n", at(float64(e[0])/60))
	}
	for _, h := range []int{10, 20, 40, 71} {
		fmt.Printf("  budget left at hour %d: %.1f%%\n", h, BudgetLeft(static, h*60)*100)
	}

	burn := series(func(h float64) float64 {
		switch {
		case blip(h, 10):
			return 2
		case h >= 30 && h < 31:
			return 2
		case h >= 45:
			return 0.3
		}
		return 0.05
	})
	fmt.Println("tab 2: burn rate")
	fmt.Printf("  normal: burn %.1f\n", Burn(burn, 300, 60))
	b := 10*60 + 1
	fmt.Printf("  blip: 5 min burn %.1f, 1 h burn %.2f, fast alert %v\n", Burn(burn, b, 5), Burn(burn, b, 60), Fast(burn, b))
	for _, e := range Edges(burn, Fast) {
		s := e[0] - 30*60
		fmt.Printf("  PAGE %d minutes into the outage: 1 h %.1f, 5 min %.1f\n", s, Burn(burn, e[0], 60), Burn(burn, e[0], 5))
		fmt.Printf("  resolved %d minutes after the rollback: 5 min %.1f, 1 h %.1f\n", e[1]-31*60, Burn(burn, e[1], 5), Burn(burn, e[1], 60))
	}
	for _, e := range Edges(burn, Slow) {
		fmt.Printf("  TICKET at %s: 6 h %.2f, 3 days %.2f\n", at(float64(e[0])/60), Burn(burn, e[0], 360), Burn(burn, e[0], 3*1440))
	}
	fmt.Printf("  at the end: 6 h %.1f, 3 days %.2f\n", Burn(burn, 72*60-1, 360), Burn(burn, 72*60-1, 3*1440))
	for _, h := range []int{10, 31, 71} {
		fmt.Printf("  budget left at hour %d: %.1f%%\n", h, BudgetLeft(burn, h*60)*100)
	}
	fmt.Printf("  a burn of 14 for an hour spends %.1f%% of the budget\n", 14.0/(30*24)*100)
}
