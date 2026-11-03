// Command cap-pacelc replays the CAP and PACELC lesson: a wallet of 100
// on two replicas, a deposit of 10, a cut cable, and a normal day.
package main

import (
	"errors"
	"fmt"
)

// Op is one change to the balance, kept so replicas can merge later.
type Op struct {
	id     string
	amount int
}

// Replica holds one copy of the wallet balance and the ops it applied.
type Replica struct {
	name    string
	balance int
	ops     []Op
}

// Apply adds an op to the balance and remembers it.
func (r *Replica) Apply(op Op) {
	r.balance += op.amount
	r.ops = append(r.ops, op)
}

// Has reports whether the replica already applied the op.
func (r *Replica) Has(id string) bool {
	for _, op := range r.ops {
		if op.id == id {
			return true
		}
	}
	return false
}

// ErrNoMajority is the CP answer when a replica cannot reach a majority.
var ErrNoMajority = errors.New("no majority: try later")

// ReadCP answers only when the replica reaches a majority of all replicas.
func ReadCP(r *Replica, reach, total int) (int, error) {
	if reach*2 <= total {
		return 0, ErrNoMajority
	}
	return r.balance, nil
}

// ReadAP always answers with the local balance, whatever its age.
func ReadAP(r *Replica) int {
	return r.balance
}

// LWW keeps the balance of the later writer and drops the other one.
func LWW(a, b *Replica, bLater bool) int {
	if bLater {
		return b.balance
	}
	return a.balance
}

// MergeOps gives each replica the ops it lacks, so both end equal.
func MergeOps(a, b *Replica) {
	var toA, toB []Op
	for _, op := range b.ops {
		if !a.Has(op.id) {
			toA = append(toA, op)
		}
	}
	for _, op := range a.ops {
		if !b.Has(op.id) {
			toB = append(toB, op)
		}
	}
	for _, op := range toA {
		a.Apply(op)
	}
	for _, op := range toB {
		b.Apply(op)
	}
}

// Link carries copies from A to B. Pending holds copies not yet delivered.
type Link struct {
	pending []Op
}

// Deliver applies every pending copy on B.
func (l *Link) Deliver(b *Replica) {
	for _, op := range l.pending {
		b.Apply(op)
	}
	l.pending = nil
}

// WriteEC applies on A, copies to B and waits for B before replying.
func WriteEC(a, b *Replica, op Op) string {
	a.Apply(op)
	b.Apply(op) // the copy and B's ok come before the reply
	return "ok"
}

// WriteEL applies on A, replies at once and queues the copy for later.
func WriteEL(a *Replica, l *Link, op Op) string {
	a.Apply(op)
	l.pending = append(l.pending, op)
	return "ok"
}

// ReadStrong delivers every pending copy before it answers.
func ReadStrong(b *Replica, l *Link) int {
	l.Deliver(b)
	return b.balance
}

func show(v int, err error) string {
	if err != nil {
		return err.Error()
	}
	return fmt.Sprint(v)
}

func main() {
	// CP: the deposit reaches A, the copy to B is lost in the cut.
	a, b := &Replica{name: "A", balance: 100}, &Replica{name: "B", balance: 100}
	a.Apply(Op{"d10", 10})
	fmt.Printf("deposit 10 on A: A %d, B %d, link cut\n", a.balance, b.balance)
	fmt.Println("CP read on B, reach 1 of 2:", show(ReadCP(b, 1, 2)))
	fmt.Println("CP read on A, reach 1 of 2:", show(ReadCP(a, 1, 2)))
	fmt.Println("fix, 3 replicas: read on A, reach 2 of 3:", show(ReadCP(a, 2, 3)),
		"| read on B, reach 1 of 3:", show(ReadCP(b, 1, 3)))

	// AP: B answers and takes writes; the heal must merge.
	a, b = &Replica{name: "A", balance: 100}, &Replica{name: "B", balance: 100}
	a.Apply(Op{"d10", 10})
	fmt.Printf("AP read on B: %d (A holds %d)\n", ReadAP(b), a.balance)
	b.Apply(Op{"w30", -30})
	fmt.Printf("withdraw 30 on B: B %d\n", b.balance)
	fmt.Printf("heal, last writer wins: %d, the deposit of 10 is lost\n", LWW(a, b, true))
	MergeOps(a, b)
	fmt.Printf("heal, merge ops: A %d, B %d\n", a.balance, b.balance)

	// PACELC: a normal day, no cut.
	a, b = &Replica{name: "A", balance: 100}, &Replica{name: "B", balance: 100}
	WriteEC(a, b, Op{"d10", 10})
	fmt.Printf("EC deposit 10: A %d, B %d, read on B %d\n", a.balance, b.balance, ReadAP(b))
	l := &Link{}
	WriteEL(a, l, Op{"d10b", 10})
	fmt.Printf("EL deposit 10: A %d, B %d, read on B %d (stale)\n", a.balance, b.balance, ReadAP(b))
	fmt.Println("fix, strong read on B:", ReadStrong(b, l))
}
