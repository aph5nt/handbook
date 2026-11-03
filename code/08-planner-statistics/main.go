// An in-memory model of a cost-based planner. It needs no database: the
// planner only ever reads statistics, so the model holds statistics and a
// cost formula, and replays the lesson's three stories.
package main

import "fmt"

// Stats is what ANALYZE stores. A missing table reads as 0 rows.
type Stats struct {
	rows map[string]int64
}

func (s Stats) Rows(table string) int64 { return s.rows[table] }

// Cost counts row touches. It is a model, not any engine's real formula.
func costNestedLoop(outer, inner int64) int64 { return outer + outer*inner }
func costHashJoin(outer, inner int64) int64   { return outer + 2*inner + 1000 }

// plan picks the cheaper join for transfers JOIN addresses.
func plan(s Stats) string {
	nOut := s.Rows("transfers")
	nIn := s.Rows("addresses")
	nl, hj := costNestedLoop(nOut, nIn), costHashJoin(nOut, nIn)
	fmt.Printf("  estimated rows: transfers %d, addresses %d\n", nOut, nIn)
	fmt.Printf("  cost nested loop = %d + %d x %d = %d\n", nOut, nOut, nIn, nl)
	fmt.Printf("  cost hash join   = %d + 2 x %d + 1000 = %d\n", nOut, nIn, hj)
	if nl <= hj {
		return "nested loop"
	}
	return "hash join"
}

const (
	transfersOfDay = 200_000
	addressesReal  = 30_000_000
)

func main() {
	fmt.Println("Tab 1: bulk load, no ANALYZE")
	s := Stats{rows: map[string]int64{"transfers": transfersOfDay}}
	p := plan(s)
	fmt.Printf("  chosen: %s\n", p)
	fmt.Printf("  real work of that plan: %d row touches\n", costNestedLoop(transfersOfDay, addressesReal))

	fmt.Println("Tab 2: ANALYZE addresses")
	s.rows["addresses"] = addressesReal
	fmt.Printf("  chosen: %s\n", plan(s))

	fmt.Println("Tab 3: correlated columns")
	const users = 30_000_000
	selCity, selPost := 1.0/10, 1.0/1000
	independent := float64(users) * selCity * selPost
	fmt.Printf("  estimate = %d x 1/10 x 1/1000 = %.0f\n", users, independent)
	// Build the column pair row by row: every 1,000th user has postcode
	// 00-950, and every 10th lives in Warsaw, so every 00-950 is in Warsaw.
	var actual int64
	for i := 0; i < users; i++ {
		warsaw := i%10 == 0
		post := i%1000 == 0
		if warsaw && post {
			actual++
		}
	}
	fmt.Printf("  actual   = %d (%.0f times more)\n", actual, float64(actual)/independent)
	// With dependency statistics the planner knows postcode implies city.
	withDeps := float64(users) * selPost
	fmt.Printf("  estimate with dependency statistics = %d x 1/1000 = %.0f\n", users, withDeps)
}
