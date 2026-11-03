// Fencing token: two indexer pods share one lane through a lease.
// Pod A pauses, pod B takes over, pod A wakes and writes a stale height.
// With the token the checkpoint refuses it. Without the token it lands.
// Run: go run .
package main

import "fmt"

const leaseSeconds = 30

// Lease is the one row that says who may write: UPDATE lease SET owner = :pod,
// epoch = epoch + 1, expires = now() + 30 seconds WHERE the lease is free.
type Lease struct {
	owner   string
	epoch   int
	expires int // seconds on the shared clock
}

// Acquire takes the lease when it is free or expired. Each take bumps the epoch.
func (l *Lease) Acquire(pod string, now int) (epoch int, ok bool) {
	if l.owner != "" && now < l.expires {
		return 0, false // someone holds a live lease
	}
	l.owner = pod
	l.epoch++
	l.expires = now + leaseSeconds
	return l.epoch, true
}

// Checkpoint is the lane's progress row: the height and the newest token that wrote it.
type Checkpoint struct {
	height int
	epoch  int
}

// FencedWrite is UPDATE checkpoint SET height = :h, epoch = :e WHERE epoch <= :e.
// It returns the number of rows changed.
func (c *Checkpoint) FencedWrite(h, e int) (rows int) {
	if c.epoch > e {
		return 0 // a newer token already wrote: refuse
	}
	c.height = h
	c.epoch = e
	return 1
}

// PlainWrite is UPDATE checkpoint SET height = :h. Nothing is checked.
func (c *Checkpoint) PlainWrite(h int) (rows int) {
	c.height = h
	return 1
}

func withToken() {
	fmt.Println("== with a fencing token")
	lease := &Lease{epoch: 6}
	cp := &Checkpoint{height: 9_400, epoch: 6}
	a, _ := lease.Acquire("A", 0)
	fmt.Printf("t=0s pod A takes the lease: epoch %d, expires at t=%ds\n", a, lease.expires)
	rows := cp.FencedWrite(9_500, a)
	fmt.Printf("pod A writes height 9500 WHERE epoch <= %d: %d row, checkpoint epoch %d\n", a, rows, cp.epoch)
	fmt.Println("t=5s pod A freezes in a 60 second pause")
	_, ok := lease.Acquire("B", 20)
	fmt.Printf("t=20s pod B asks for the lease: granted %v, still live\n", ok)
	b, ok := lease.Acquire("B", 40)
	fmt.Printf("t=40s pod B takes the expired lease: granted %v, epoch %d\n", ok, b)
	rows = cp.FencedWrite(9_600, b)
	fmt.Printf("pod B writes height 9600 WHERE epoch <= %d: %d row, checkpoint epoch %d\n", b, rows, cp.epoch)
	rows = cp.FencedWrite(9_506, a)
	fmt.Printf("t=65s pod A wakes, writes 9506 WHERE epoch <= %d: %d rows, height stays %d\n", a, rows, cp.height)
	if rows == 0 {
		fmt.Println("pod A reads 0 rows as: no longer the writer. It stops and goes to standby.")
	}
}

func noToken() {
	fmt.Println("== same lane, no token")
	lease := &Lease{epoch: 6}
	cp := &Checkpoint{height: 9_400}
	lease.Acquire("A", 0)
	cp.PlainWrite(9_500)
	fmt.Printf("pod A holds the lease, writes height %d with no epoch in the WHERE\n", cp.height)
	lease.Acquire("B", 40)
	cp.PlainWrite(9_600)
	fmt.Printf("pod A pauses, pod B takes the lease, writes height %d\n", cp.height)
	rows := cp.PlainWrite(9_506)
	fmt.Printf("pod A wakes, writes 9506: %d row, the checkpoint goes back to %d\n", rows, cp.height)
	seen := []int{}
	for i := 1; i <= 2; i++ {
		cp.PlainWrite(9_506 + i) // pod A goes on from its own height
		seen = append(seen, cp.height)
		cp.PlainWrite(9_600 + i) // pod B goes on from its own height
		seen = append(seen, cp.height)
	}
	fmt.Printf("two writers: the checkpoint jumps %v\n", seen)
}

func main() {
	withToken()
	noToken()
}
