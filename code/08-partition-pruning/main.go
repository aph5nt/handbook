// An in-memory model of partition and column pruning. The table is 30 day
// partitions of 12 column files each, and only its metadata is modelled. The
// planner decides from metadata, so no data is needed to prove the numbers.
package main

import "fmt"

const (
	days    = 30
	columns = 12
)

// Pred is a filter. Bare says the partition column stands alone in it.
type Pred struct {
	Bare   bool
	Lo, Hi int // a range of days of October
}

// prune is the planner's check against each partition's day.
func prune(p *Pred) []int {
	var keep []int
	for day := 1; day <= days; day++ {
		switch {
		case p == nil, !p.Bare: // no filter, or a filter it cannot prove anything from
			keep = append(keep, day)
		case day >= p.Lo && day <= p.Hi:
			keep = append(keep, day)
		}
	}
	return keep
}

func report(label string, p *Pred, cols int) {
	parts := len(prune(p))
	files := parts * cols
	fmt.Printf("%-44s %2d partitions x %2d columns = %3d files, %5.1f%% of the table\n",
		label, parts, cols, files, 100*float64(files)/float64(days*columns))
}

func main() {
	r := &Pred{Bare: true, Lo: 10, Hi: 12}
	report("SELECT * with no filter", nil, columns)
	report("SELECT *, day BETWEEN 10 and 12", r, columns)
	report("SELECT *, CAST(day AS VARCHAR) BETWEEN ...", &Pred{Lo: 10, Hi: 12}, columns)
	report("SELECT *, filter only on the joined table", &Pred{Lo: 10, Hi: 12}, columns)
	report("SELECT to_address, SUM(amount), day range", r, 2)
	report("SELECT * LIMIT 10, no filter", nil, columns)
	fmt.Printf("over two years of days: %d of %d partitions\n", 3, 730)
}
