// An in-memory model of reading a plan. It needs no database: it holds the
// plan tree that EXPLAIN ANALYZE would print (estimated rows, actual rows,
// partitions read, rows moved over the network) and walks it the way the
// lesson does, leaves first, stopping at the first wrong operator.
package main

import "fmt"

const nodes = 4 // backend nodes in the warehouse

// Op is one operator line of an EXPLAIN ANALYZE plan.
type Op struct {
	Name        string
	Est, Actual int64
	Scan        bool
	PartsRead   int
	PartsTotal  int
	Join        bool
	RowsMoved   int64
	Spilled     bool
}

// chooseJoin is the planner's rule for a distributed join.
func chooseJoin(big, small int64, colocated bool) (string, int64) {
	if colocated {
		return "colocate", 0 // both sides already bucketed on the key
	}
	broadcast := small * nodes // copy the small side to every node
	shuffle := big + small     // re-send both sides by the join key
	fmt.Printf("    broadcast = %d x %d = %d rows moved, shuffle = %d + %d = %d rows moved\n",
		small, nodes, broadcast, big, small, shuffle)
	if broadcast <= shuffle {
		return "broadcast", broadcast
	}
	return "shuffle", shuffle
}

// diagnose walks the operators leaves first and returns the first finding.
func diagnose(plan []Op) string {
	for _, op := range plan {
		if op.Actual > 10*op.Est || op.Est > 10*op.Actual {
			return op.Name + ": estimate wrong, refresh statistics or fix the predicate"
		}
		if op.Scan && op.PartsTotal > 1 && op.PartsRead == op.PartsTotal {
			return op.Name + ": full scan, fix the predicate or the key"
		}
		if op.Join && op.RowsMoved > 1_000_000 {
			return fmt.Sprintf("%s: moves %d rows per statement, colocate or hint", op.Name, op.RowsMoved)
		}
		if op.Spilled {
			return op.Name + ": spilled, cut rows earlier or raise memory"
		}
	}
	return "every operator fine: look at round trips and bytes read"
}

func run(title string, poolsEst, poolsActual int64, colocated bool) {
	fmt.Println(title)
	kind, moved := chooseJoin(4_000_000, poolsEst, colocated)
	fmt.Printf("    chosen join: %s, %d rows moved\n", kind, moved)
	plan := []Op{
		{Name: "Scan swaps", Est: 4_000_000, Actual: 4_100_000, Scan: true, PartsRead: 1, PartsTotal: 365},
		{Name: "Scan pools", Est: poolsEst, Actual: poolsActual, Scan: true, PartsRead: 1, PartsTotal: 1},
		{Name: "Hash join (" + kind + ")", Est: 4_000_000, Actual: 4_100_000, Join: true, RowsMoved: moved},
	}
	for _, op := range plan {
		fmt.Printf("    %-26s est %9d  actual %9d\n", op.Name, op.Est, op.Actual)
	}
	fmt.Println("    diagnosis:", diagnose(plan))
}

func main() {
	run("Yesterday: pools holds 2,000 rows", 2_000, 2_100, false)
	run("Today: pools grew to 2.1 million rows", 2_100_000, 2_100_000, false)
	run("Fix: both tables colocated on pool", 2_100_000, 2_100_000, true)
}
