// Split brain, leases and fencing tokens. Two indexer pods compete for block
// range 1000-1999. Pod 1 takes the lease, freezes in a garbage-collection
// pause, and wakes up after pod 2 took over. Without a fencing check the
// warehouse accepts pod 1's stale write. With the check it refuses token 33.
package main

import (
	"fmt"
	"time"
)

// LeaseStore hands out one lease at a time, each with a growing token.
type LeaseStore struct {
	holder  string
	expires time.Time
	token   int
}

// Acquire grants the lease if it is free or expired. The token only grows.
func (s *LeaseStore) Acquire(pod string, now time.Time, ttl time.Duration) (int, bool) {
	if s.holder != "" && now.Before(s.expires) {
		return 0, false // someone else holds a live lease
	}
	s.token++ // 33, then 34: a newer lease always has a bigger number
	s.holder, s.expires = pod, now.Add(ttl)
	return s.token, true
}

// Warehouse stores the rows of one block range.
type Warehouse struct {
	fencing bool   // check the token on every write?
	highest int    // the highest token seen so far
	rowsBy  string // which pod wrote the rows now stored
	damaged bool   // a stale owner wrote over a newer owner's rows
}

// Write stores a batch. With fencing on, a token below the highest is refused.
func (w *Warehouse) Write(pod string, token int) bool {
	if w.fencing && token < w.highest {
		return false // stale owner: 33 is older than 34
	}
	if token < w.highest {
		w.damaged = true // split brain: the old owner overwrote the new one
	}
	if token > w.highest {
		w.highest = token
	}
	w.rowsBy = pod
	return true
}

func run(fencing bool) {
	t0 := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	ls := &LeaseStore{token: 32}
	wh := &Warehouse{fencing: fencing}
	t1, _ := ls.Acquire("pod 1", t0, 10*time.Second)
	fmt.Printf("pod 1 acquires: token %d, lease 10 s\n", t1)
	fmt.Printf("pod 1 writes token %d: accepted %v, highest %d\n", t1, wh.Write("pod 1", t1), wh.highest)
	fmt.Println("pod 1 freezes for 15 s, no renewal is sent")
	if _, ok := ls.Acquire("pod 2", t0.Add(5*time.Second), 10*time.Second); !ok {
		fmt.Println("pod 2 at 5 s: refused, the lease is still live")
	}
	t2, _ := ls.Acquire("pod 2", t0.Add(11*time.Second), 10*time.Second)
	fmt.Printf("pod 2 at 11 s: lease expired, token %d\n", t2)
	fmt.Printf("pod 2 writes token %d: accepted %v, highest %d\n", t2, wh.Write("pod 2", t2), wh.highest)
	ok := wh.Write("pod 1", t1)
	fmt.Printf("pod 1 wakes at 15 s, writes token %d: accepted %v, rows by %s, damaged %v\n", t1, ok, wh.rowsBy, wh.damaged)
}

func main() {
	fmt.Println("== Lease only (fencing off)")
	run(false)
	fmt.Println("== Lease plus fencing token")
	run(true)
}
