// Replays the observability lesson: one page on the symptom, a board for the
// causes, then the team that pages on every cause.
package main

import "fmt"

// Symptom is the one alert users would ask for: stale data, for long enough.
type Symptom struct {
	Limit, ForMinutes, over int
}

// Check runs once a minute and says whether to page this minute.
func (a *Symptom) Check(behind int) bool {
	if behind <= a.Limit {
		a.over = 0
		return false
	}
	a.over++
	return a.over == a.ForMinutes // once, not every minute after
}

// Tile is one cause on the dashboard. It pages nobody.
type Tile struct {
	Name         string
	Value, Limit int
}

// Diagnose reads the board and names what is over its line.
func Diagnose(board []Tile) []string {
	var red []string
	for _, t := range board {
		if t.Value > t.Limit {
			red = append(red, t.Name)
		}
	}
	return red
}

// Slowest reads one trace and names the span where the time went.
func Slowest(spans map[string]int) (string, int) {
	worst, ms := "", 0
	for name, t := range spans {
		if t > ms {
			worst, ms = name, t
		}
	}
	return worst, ms
}

// LabelOK keeps metric labels few. More values belong in logs and traces.
func LabelOK(values int) bool {
	return values <= 100
}

// Route decides page or ticket from how soon the 30-day budget runs out.
func Route(burn float64) string {
	hoursLeft := 30 * 24 / burn
	if hoursLeft < 24 {
		return "page"
	}
	return "ticket"
}

// CausePages is the other team: every rule over its line is a page.
func CausePages(rules []Tile) int {
	pages := 0
	for _, r := range rules {
		if r.Value > r.Limit {
			pages++
		}
	}
	return pages
}

// Worth is the test for keeping a page: did a person have to act?
func Worth(neededAction bool) string {
	if !neededAction {
		return "delete it: it teaches people to ignore the pager"
	}
	return "keep"
}

func main() {
	alert := &Symptom{Limit: 1000, ForMinutes: 10}
	pages := 0
	for m := 0; m < 30; m++ {
		behind := 20
		if m >= 5 {
			behind = 1200
		}
		if alert.Check(behind) {
			pages++
			fmt.Printf("minute %d: PAGE, data %d blocks behind for %d minutes\n", m, behind, alert.ForMinutes)
		}
	}
	fmt.Println("pages for the incident:", pages)
	board := []Tile{{"cpu", 30, 90}, {"disk", 98, 90}, {"queue", 4000, 1000}, {"node lag", 0, 100}}
	fmt.Println("board, over the line:", Diagnose(board))
	spans := map[string]int{"read node": 40, "decode": 12, "warehouse insert": 4000}
	worst, ms := Slowest(spans)
	fmt.Println("log: write timeout to warehouse. trace: longest span is", worst, ms, "ms")
	for _, b := range []float64{1000, 1.5} {
		fmt.Printf("burn %6.1f: budget gone in %6.1f hours: %s\n", b, 720/b, Route(b))
	}

	rules := make([]Tile, 20)
	for k := range rules {
		rules[k] = Tile{fmt.Sprintf("rule%d", k), 2, 1}
	}
	fmt.Println("\ntab 2: same slow disk, a rule per cause:", CausePages(rules), "pages")
	fmt.Println("cpu spike that clears by itself:", Worth(false))
	fmt.Println("label lane, 2 values, ok:", LabelOK(2), "; label address, 40,000,000 values, ok:", LabelOK(40_000_000))
	fmt.Println("stale data for 10 minutes:", Worth(true))
}
