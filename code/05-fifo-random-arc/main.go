// FIFO, random and ARC on the same reads: A, B, A, C, D, then a scan
// S1, S2, then A again. Four slots. Which policy keeps the hot key A?
package main

import (
	"fmt"
	"math/rand"
	"slices"
)

var reads = []string{"A", "B", "A", "C", "D", "S1", "S2", "A"}

// FIFO: a queue in insert order. A read of a cached key changes nothing.
func fifo(reads []string) {
	var q []string
	for _, k := range reads {
		if slices.Contains(q, k) {
			fmt.Printf("  read %-2s hit   %v\n", k, q)
			continue
		}
		if len(q) == 4 {
			fmt.Printf("  evict %s (first in)\n", q[0])
			q = q[1:] // the oldest insert goes, however hot it is
		}
		q = append(q, k)
		fmt.Printf("  read %-2s miss  %v\n", k, q)
	}
}

// Random: no order and no counters. pick chooses the victim's slot.
func random(reads []string, pick func(n int) int) {
	var slots []string
	for _, k := range reads {
		if slices.Contains(slots, k) {
			fmt.Printf("  read %-2s hit   %v\n", k, slots)
			continue
		}
		if len(slots) == 4 {
			v := pick(4)
			fmt.Printf("  evict %s (by chance)\n", slots[v])
			slots[v] = k // the new key takes the victim's slot
		} else {
			slots = append(slots, k)
		}
		fmt.Printf("  read %-2s miss  %v\n", k, slots)
	}
}

// ARC: recent (seen once) and frequent (seen twice or more), plus ghost
// lists of evicted names. p is the target size of the recent list.
type ARC struct {
	c, p           int
	t1, t2, b1, b2 []string // front = least recently used
}

func remove(l []string, k string) []string {
	i := slices.Index(l, k)
	return append(l[:i], l[i+1:]...)
}

func (a *ARC) replace(inB2 bool) {
	if len(a.t1) > 0 && (len(a.t1) > a.p || (inB2 && len(a.t1) == a.p)) {
		v := a.t1[0] // evict from recent, keep its name as a ghost
		a.t1, a.b1 = a.t1[1:], append(a.b1, v)
		fmt.Printf("  evict %s from recent, ghost kept\n", v)
	} else {
		v := a.t2[0]
		a.t2, a.b2 = a.t2[1:], append(a.b2, v)
		fmt.Printf("  evict %s from frequent, ghost kept\n", v)
	}
}

func (a *ARC) Read(k string) {
	switch {
	case slices.Contains(a.t1, k) || slices.Contains(a.t2, k): // hit
		if slices.Contains(a.t1, k) {
			a.t1 = remove(a.t1, k)
		} else {
			a.t2 = remove(a.t2, k)
		}
		a.t2 = append(a.t2, k) // seen twice or more: frequent
		fmt.Printf("  read %-2s HIT   recent=%v frequent=%v\n", k, a.t1, a.t2)
		return
	case slices.Contains(a.b1, k): // a ghost of recent: recent was too small
		a.p = min(a.c, a.p+1)
		a.replace(false)
		a.b1 = remove(a.b1, k)
		a.t2 = append(a.t2, k)
	case slices.Contains(a.b2, k): // a ghost of frequent: frequent was too small
		a.p = max(0, a.p-1)
		a.replace(true)
		a.b2 = remove(a.b2, k)
		a.t2 = append(a.t2, k)
	default: // brand new key
		if len(a.t1)+len(a.t2) == a.c {
			a.replace(false)
		}
		a.t1 = append(a.t1, k)
	}
	fmt.Printf("  read %-2s miss  recent=%v frequent=%v ghosts=%v p=%d\n", k, a.t1, a.t2, a.b1, a.p)
}

func main() {
	fmt.Println("== FIFO ==")
	fifo([]string{"A", "B", "A", "C", "D", "S1", "S2", "A"})
	fmt.Println("== Random, the lucky run: the coin picks C, then D ==")
	coin := []int{2, 3}
	random(reads, func(int) int { v := coin[0]; coin = coin[1:]; return v })
	fmt.Println("== Random, another run: the coin picks A first ==")
	coin = []int{0, 3, 1}
	random(reads, func(int) int { v := coin[0]; coin = coin[1:]; return v })
	fmt.Println("== Random with a real generator, seed 7 ==")
	r := rand.New(rand.NewSource(7))
	random(reads, r.Intn)
	fmt.Println("== ARC ==")
	a := &ARC{c: 4}
	for _, k := range append(reads, "B") {
		a.Read(k)
	}
}
