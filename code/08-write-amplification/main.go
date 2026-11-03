// A byte counter, not a storage engine. The sizes and the levelled and tiered
// counts are assumptions stated below. It shows the worst case of a B-tree:
// each change flushed alone. Real engines batch page writes, so they do better.
package main

import "fmt"

const (
	appBytes = 100
	wal      = 150  // a log record for the change
	page     = 8192 // a page
)

func btree(indexes int) int { return wal + page + indexes*page }

func ratio(written int) float64 { return float64(written) / appBytes }

func main() {
	fmt.Printf("B-tree, %d bytes changed:\n", appBytes)
	fmt.Printf("  log only:                 %5d bytes, %6.1fx\n", wal, ratio(wal))
	fmt.Printf("  log + table page:         %5d bytes, %6.1fx\n", btree(0), ratio(btree(0)))
	fmt.Printf("  log + page + 5 indexes:   %5d bytes, %6.1fx\n", btree(5), ratio(btree(5)))
	fmt.Printf("  2 unused indexes dropped: %5d bytes, %6.1fx\n", btree(3), ratio(btree(3)))
	fmt.Printf("  a HOT update, no indexes: %5d bytes, %6.1fx\n", btree(0), ratio(btree(0)))

	// LSM: 1 byte goes to the log, 1 to the flush, then compaction rewrites it.
	const levels, perLevel = 3, 10
	levelled := 1 + 1 + levels*perLevel
	tiered := 1 + 1 + levels
	const tinyRounds = 5 // 1,000 one-row files, merged four at a time, take about five rounds
	fmt.Printf("LSM, per byte:\n")
	fmt.Printf("  log + flush:              %dx\n", 2)
	fmt.Printf("  levelled, %d levels x %d:   %dx\n", levels, perLevel, levelled)
	fmt.Printf("  plus tiny-file merges:    %dx\n", levelled+tinyRounds)
	fmt.Printf("  tiered, big batches:      %dx\n", tiered)
}
