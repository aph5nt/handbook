// Linearizable versus serializable.
// Seat store: Kim books 7A on the leader; a lagging replica still says free,
// which breaks linearizability; reading through the leader fixes it.
// Bank: T1 moves 50 from Kim to Lee, T2 sums balances. Without isolation T2
// sees half of T1 (100). Serializable gives 150. A replica snapshot is still
// serializable but old. Strict serializable reads after T1 in real time.
package main

import (
	"fmt"
	"sync"
)

// SeatStore has a leader and one replica that copies writes later.
type SeatStore struct {
	leader, replica map[string]string
}

// Book writes on the leader. The replica gets it later, in Replicate.
func (s *SeatStore) Book(seat, who string) {
	s.leader[seat] = who
}

func (s *SeatStore) Replicate() {
	for k, v := range s.leader {
		s.replica[k] = v
	}
}

// ReadReplica may be behind: not linearizable.
func (s *SeatStore) ReadReplica(seat string) string { return orFree(s.replica[seat]) }

// ReadLeader sees every finished write: linearizable.
func (s *SeatStore) ReadLeader(seat string) string { return orFree(s.leader[seat]) }

func orFree(v string) string {
	if v == "" {
		return "free"
	}
	return v
}

// Bank holds two balances and a lock for serializable transactions.
type Bank struct {
	mu       sync.Mutex
	kim, lee int
}

// Transfer is T1. With isolation it holds the lock for the whole transaction.
// between runs after the debit and before the credit: where T2 can sneak in.
func (b *Bank) Transfer(isolated bool, between func()) {
	if isolated {
		b.mu.Lock()
		defer b.mu.Unlock()
	}
	b.kim -= 50
	between()
	b.lee += 50
}

// Report is T2. With isolation it waits for the lock, so it never sees half of T1.
func (b *Bank) Report(isolated bool) int {
	if isolated {
		b.mu.Lock()
		defer b.mu.Unlock()
	}
	return b.kim + b.lee
}

func main() {
	fmt.Println("== Linearizable: seat 7A")
	s := &SeatStore{leader: map[string]string{}, replica: map[string]string{}}
	s.Book("7A", "Kim")
	fmt.Printf("12:00:03, after Kim's write ended, replica read: %s: seat sold twice\n", s.ReadReplica("7A"))
	fmt.Printf("fix, read through the leader: %s\n", s.ReadLeader("7A"))
	s.Replicate()
	fmt.Printf("later the replica catches up: %s\n", s.ReadReplica("7A"))

	fmt.Println("== Serializable: transfer and report")
	b := &Bank{kim: 100, lee: 50}
	var seen int
	b.Transfer(false, func() { seen = b.Report(false) })
	fmt.Printf("no isolation, report between debit and credit: %d\n", seen)

	b = &Bank{kim: 100, lee: 50}
	done := make(chan int)
	b.Transfer(true, func() {
		go func() { done <- b.Report(true) }() // T2 starts now and must wait
	})
	fmt.Printf("serializable, report waited for T1: %d\n", <-done)

	b = &Bank{kim: 100, lee: 50}
	snapKim, snapLee := b.kim, b.lee // a replica snapshot taken before T1
	b.Transfer(true, func() {})
	fmt.Printf("replica snapshot after T1 ended: kim %d + lee %d = %d: valid order, old data\n", snapKim, snapLee, snapKim+snapLee)
	fmt.Printf("strict serializable, read after T1: kim %d + lee %d = %d\n", b.kim, b.lee, b.Report(true))
}
