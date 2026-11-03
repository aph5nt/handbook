// Log broker versus queue broker, and a compacted topic.
//
// Slices stand in for a Kafka partition and a RabbitMQ queue. The log keeps
// records and each reader keeps its own offset. The queue deletes a message
// on ack. Deterministic.
package main

import "fmt"

// Log: reading does not remove anything.
type Log struct{ recs []string }

func (l *Log) Read(off int) (string, bool) {
	if off >= len(l.recs) {
		return "", false
	}
	return l.recs[off], true
}

// Queue: an ack removes the message for everyone.
type Queue struct{ msgs []string }

func (q *Queue) Ack() { q.msgs = q.msgs[1:] }

type Rec struct{ Key, Val string }

// compact keeps the newest record per key, in the order of those newest records.
func compact(in []Rec) []Rec {
	last := map[string]int{}
	for i, r := range in {
		last[r.Key] = i
	}
	var out []Rec
	for i, r := range in {
		if last[r.Key] == i {
			out = append(out, r)
		}
	}
	return out
}

// process is consumer A's work on one message.
func process(m string) {
	fmt.Println("consumer A processed", m)
}

func main() {
	events := []string{"e0", "e1", "e2", "e3", "e4", "e5", "e6", "e7"}

	fmt.Println("== Tab 1: log ==")
	l := &Log{events}
	a, b := 8, 0 // reader A is at the end, reader B starts at 0
	for {
		if _, ok := l.Read(b); !ok {
			break
		}
		b++
	}
	fmt.Printf("reader B replayed %d events, offsets A=%d B=%d, log still holds %d\n", b, a, b, len(l.recs))

	fmt.Println("== Tab 2: queue ==")
	q := &Queue{append([]string{}, events...)}
	acked := 0
	for len(q.msgs) > 0 {
		process(q.msgs[0])
		q.Ack()
		acked++
	}
	n := len(q.msgs) // consumer B connects now: 0
	fmt.Printf("consumer A acked %d, the queue holds %d: consumer B finds nothing\n", acked, n)

	fmt.Println("== Tab 3: compacted topic ==")
	log := []Rec{{"A", "v1"}, {"B", "v1"}, {"C", "v1"}, {"A", "v2"}, {"B", ""}, {"D", "v1"}}
	c := compact(log)
	fmt.Printf("compacted from %d to %d: %v\n", len(log), len(c), c)
	table := map[string]string{}
	for _, r := range c {
		if r.Val == "" {
			delete(table, r.Key) // a tombstone
			continue
		}
		table[r.Key] = r.Val
	}
	fmt.Printf("a new reader's table: %v\n", table)
	fmt.Printf("what was A before v2? gone: %v\n", func() bool {
		for _, r := range c {
			if r.Key == "A" && r.Val == "v1" {
				return false
			}
		}
		return true
	}())
}
