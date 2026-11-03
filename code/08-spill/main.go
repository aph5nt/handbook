// The two sorts of the lesson, written out in memory: an external merge sort
// with a budget of 4 rows (the "disk" is a slice of runs we count writes and
// reads on), and a top-N sort that keeps only the best 3 rows in a min-heap.
package main

import (
	"container/heap"
	"fmt"
	"sort"
)

var amounts = []int{41, 7, 93, 12, 58, 33, 88, 5, 71, 20, 64, 99}

const budget = 4

// externalSort spills a sorted run each time memory holds `budget` rows,
// then merges the runs, biggest first.
func externalSort(rows []int) ([]int, int, int) {
	var runs [][]int
	written, read := 0, 0
	buf := []int{}
	for _, r := range rows {
		buf = append(buf, r)
		if len(buf) == budget { // memory full: spill
			sort.Ints(buf)
			runs = append(runs, buf)
			written += len(buf)
			fmt.Printf("  spill run %d: %v\n", len(runs), buf)
			buf = []int{}
		}
	}
	if len(buf) > 0 {
		sort.Ints(buf)
		runs = append(runs, buf)
		written += len(buf)
	}
	// merge: repeatedly take the biggest tail of all runs
	var out []int
	for {
		best := -1
		for i, run := range runs {
			if len(run) > 0 && (best < 0 || run[len(run)-1] > runs[best][len(runs[best])-1]) {
				best = i
			}
		}
		if best < 0 {
			break
		}
		r := runs[best]
		out = append(out, r[len(r)-1])
		runs[best] = r[:len(r)-1]
		read++
	}
	return out, written, read
}

type minHeap []int

func (h minHeap) Len() int           { return len(h) }
func (h minHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h minHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *minHeap) Push(x any)        { *h = append(*h, x.(int)) }
func (h *minHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

func sortedDesc(h minHeap) []int {
	s := append([]int{}, h...)
	sort.Sort(sort.Reverse(sort.IntSlice(s)))
	return s
}

// topN keeps the n biggest rows. A smaller row is dropped at once.
func topN(rows []int, n int) []int {
	h := &minHeap{}
	for i, r := range rows {
		switch {
		case h.Len() < n:
			heap.Push(h, r)
		case r > (*h)[0]:
			fmt.Printf("  row %2d: %2d pushes out %2d\n", i+1, r, (*h)[0])
			(*h)[0] = r
			heap.Fix(h, 0)
		default:
			fmt.Printf("  row %2d: %2d dropped, below %2d\n", i+1, r, (*h)[0])
		}
		if i == 3 || i == 7 || i == 11 {
			fmt.Printf("  after %d rows kept: %v\n", i+1, sortedDesc(*h))
		}
	}
	return sortedDesc(*h)
}

func main() {
	fmt.Printf("Full sort of %d rows, memory for %d:\n", len(amounts), budget)
	out, w, r := externalSort(amounts)
	fmt.Printf("  output %v\n  rows written to disk %d, read back %d\n", out, w, r)
	fmt.Println("Top 3 with a heap, memory for 3:")
	fmt.Printf("  output %v, rows written to disk 0\n", topN(amounts, 3))
}
