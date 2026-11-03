// Idempotent producer and Kafka transactions, replayed step by step.
//
// A Go struct stands in for one Kafka partition. It remembers the last
// sequence number written per producer id, as a broker does, and marks
// transactional records as pending until a commit or abort marker.
// Deterministic.
package main

import "fmt"

type Rec struct {
	Val   string
	TxnID int // 0 = not transactional
}

type Partition struct {
	log     []Rec
	lastSeq map[int]int // producer id -> last sequence written
	status  map[int]string
}

// Append is the broker's idempotence check.
func (p *Partition) Append(pid, seq int, r Rec) string {
	last, seen := p.lastSeq[pid]
	switch {
	case seen && seq <= last:
		return "duplicate, dropped, acked" // already written
	case seen && seq != last+1:
		return "out of order, refused" // a gap: wait for the missing one
	}
	p.lastSeq[pid] = seq
	p.log = append(p.log, r)
	return "written"
}

// Read returns what a consumer sees with the given isolation level.
func (p *Partition) Read(committedOnly bool) []string {
	var out []string
	for _, r := range p.log {
		if committedOnly && r.TxnID != 0 && p.status[r.TxnID] != "committed" {
			continue // pending or aborted: hidden by read_committed
		}
		out = append(out, r.Val)
	}
	return out
}

func main() {
	fmt.Println("== Tab 1: idempotent producer ==")
	p := &Partition{lastSeq: map[int]int{42: 6}, status: map[int]string{}}
	fmt.Println("send pid 42 seq 7:", p.Append(42, 7, Rec{Val: "swap seq 7"}))
	fmt.Println("ack lost, retry pid 42 seq 7:", p.Append(42, 7, Rec{Val: "swap seq 7"}))
	q := &Partition{lastSeq: map[int]int{42: 6}, status: map[int]string{}}
	fmt.Println("seq 8 arrives before the retried 7:", q.Append(42, 8, Rec{Val: "swap seq 8"}))
	q.Append(42, 7, Rec{Val: "swap seq 7"})
	q.Append(42, 8, Rec{Val: "swap seq 8"})
	fmt.Println("log after the retry:", q.Read(false))

	fmt.Println("== Tab 2: transactions ==")
	out := &Partition{lastSeq: map[int]int{}, status: map[int]string{}}
	offsets := map[int]string{} // the consumer offset commit, part of the txn
	out.status[1] = "pending"
	out.Append(7, 1, Rec{"swap A", 1})
	out.Append(7, 2, Rec{"swap B", 1})
	offsets[1] = "input offset 501"
	fmt.Println("before commit, read_committed sees:", out.Read(true))
	out.status[1] = "committed"
	fmt.Println("after commit, read_committed sees:", out.Read(true), "and", offsets[1])
	out.status[2] = "pending"
	out.Append(7, 3, Rec{"swap C", 2})
	out.status[2] = "aborted" // the decoder crashed before commit
	fmt.Println("txn 2 aborted, read_committed sees:", out.Read(true))

	fmt.Println("== Tab 3: the edge ==")
	fmt.Println("read_uncommitted sees:", out.Read(false))
	plain := []string{}
	plain = append(plain, "0xabc:3") // first attempt, Kafka part aborted
	plain = append(plain, "0xabc:3") // redo after the crash
	fmt.Printf("Postgres without a key: %d rows for one swap\n", len(plain))
	keyed := map[string]bool{}
	for _, k := range []string{"0xabc:3", "0xabc:3"} {
		keyed[k] = true // INSERT ... ON CONFLICT (tx_hash, log_index) DO NOTHING
	}
	fmt.Printf("Postgres with unique (tx_hash, log_index): %d row\n", len(keyed))
}
