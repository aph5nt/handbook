// Re-index from genesis: a decoder bug wrote wrong swaps on days 20 to 40.
// Compare patching rows in place with a full lane reset and re-walk.
package main

import "fmt"

const days = 100

// Each store holds one value per day: "ok", "bad", "patched" or "" (empty).
type Lane struct {
	Facts, Graph, Summary [days + 1]string
	GraphMark, SumMark    int // the furthest day each reader has processed
	Decoder               int // 1 has the bug on days 20..40, 2 is fixed
	Running               map[string]bool
}

func decode(decoder, day int) string {
	if decoder == 1 && day >= 20 && day <= 40 {
		return "bad"
	}
	if decoder == 1 && day > 40 {
		return "bad" // the new exchange keeps trading: v1 stays wrong
	}
	return "ok"
}

// Walk is the indexer: it writes facts keyed by day, so a replay overwrites.
func (l *Lane) Walk(from, to int) {
	for d := from; d <= to; d++ {
		l.Facts[d] = decode(l.Decoder, d)
	}
}

// Follow is a reader: it resumes after its mark and never re-reads.
func (l *Lane) Follow(store *[days + 1]string, mark *int, to int) {
	for d := *mark + 1; d <= to; d++ {
		store[d] = l.Facts[d]
		*mark = d
	}
}

// Patch fixes rows in place while every writer and reader keeps running.
func (l *Lane) Patch(from, to int) {
	for d := from; d <= to; d++ {
		l.Facts[d] = "patched" // an UPDATE, no code change behind it
	}
}

// Reset stops everything, drops everything, and re-walks from day 1.
func (l *Lane) Reset(decoder int) error {
	for _, name := range []string{"indexer", "graph sync", "summary", "migrate"} {
		l.Running[name] = false // stop every writer and every reader
	}
	for name, on := range l.Running {
		if on {
			return fmt.Errorf("%s still running: a reset is all or nothing", name)
		}
	}
	l.Facts, l.Graph, l.Summary = [days + 1]string{}, [days + 1]string{}, [days + 1]string{}
	l.GraphMark, l.SumMark = 0, 0 // drop whole tables and marks
	l.Decoder = decoder           // image v2, tested on a sealed fixture
	l.Walk(1, days)               // re-walk from block 1
	l.Follow(&l.Graph, &l.GraphMark, days)
	l.Follow(&l.Summary, &l.SumMark, days)
	return nil
}

func count(s [days + 1]string, v string) int {
	n := 0
	for d := 1; d <= days; d++ {
		if s[d] == v {
			n++
		}
	}
	return n
}

func report(name string, l *Lane) {
	fmt.Printf("%-28s facts ok %3d bad %3d patched %3d | graph ok %3d bad %3d | summary ok %3d bad %3d\n",
		name, count(l.Facts, "ok"), count(l.Facts, "bad"), count(l.Facts, "patched"),
		count(l.Graph, "ok"), count(l.Graph, "bad"), count(l.Summary, "ok"), count(l.Summary, "bad"))
}

func newLane() *Lane {
	l := &Lane{Decoder: 1, Running: map[string]bool{"indexer": true, "graph sync": true, "summary": true, "migrate": true}}
	l.Walk(1, 40)
	l.Follow(&l.Graph, &l.GraphMark, 38)
	l.Follow(&l.Summary, &l.SumMark, 40)
	return l
}

func main() {
	l := newLane()
	report("walk reaches day 40", l)
	l.Patch(20, 40)
	l.Walk(41, 45) // the indexer still runs decoder v1
	l.Follow(&l.Graph, &l.GraphMark, 43)
	l.Follow(&l.Summary, &l.SumMark, 45)
	report("patched in place, day 45", l)
	fmt.Printf("graph mark %d: days 20..38 are never read again\n", l.GraphMark)

	l = newLane()
	if err := l.Reset(2); err != nil {
		fmt.Println("refused:", err)
		return
	}
	report("reset and re-walk, day 100", l)
}
