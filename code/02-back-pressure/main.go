// Replays the back-pressure lesson: the graph sync reads pages from
// StarRocks and writes them into DozerDB. First the reader pushes with no
// limit. Then the writer pulls two pages at a time.
// Time moves in ticks. The reader can read one page per tick.
package main

import "fmt"

const (
	slowFrom   = 10 // DozerDB starts its big merges at this tick
	slowUntil  = 60 // the merges finish at this tick (pull run only)
	timeout    = 20 // a write of a page older than this many ticks times out
	memLimit   = 60 // pages that fit in memory
	pullSize   = 2  // the writer asks for this many pages
	normalCost = 1  // ticks to write one page
	slowCost   = 4  // ticks to write one page during big merges
)

type page struct{ id, readAt int }

type Sync struct {
	pull     bool
	buf      []page // pages waiting in memory
	demand   int    // pages the writer asked for and has not got yet
	busy     int    // ticks left on the current write
	cur      page
	read     int
	written  int
	retries  int
	idle     int // ticks the reader waited for a request
	maxBuf   int
	firstOut int // tick of the first timeout
}

func cost(t int, recovers bool) int {
	if t >= slowFrom && (!recovers || t < slowUntil) {
		return slowCost // big merges in DozerDB
	}
	return normalCost
}

// tick runs the reader, then the writer, for one tick.
func (s *Sync) tick(t int) {
	if !s.pull || s.demand > 0 { // push reads always, pull only on request
		s.buf = append(s.buf, page{s.read, t})
		s.read++
		if s.pull {
			s.demand--
		}
	} else {
		s.idle++ // no request, no read
	}
	if s.busy == 0 && len(s.buf) > 0 { // the writer takes the oldest page
		s.cur, s.buf = s.buf[0], s.buf[1:]
		s.busy = cost(t, s.pull) // 4 ticks during big merges, else 1
	}
	if s.busy > 0 {
		s.busy--
		if s.busy == 0 {
			s.finish(t)
		}
	}
	if s.pull && len(s.buf) == 0 && s.demand == 0 {
		s.demand = pullSize // ask for 2 more pages
	}
	if len(s.buf) > s.maxBuf {
		s.maxBuf = len(s.buf)
	}
}

// finish ends one write. A page that waited too long times out and is
// queued again: the work was wasted and a retry is added.
func (s *Sync) finish(t int) {
	if t-s.cur.readAt > timeout {
		s.retries++
		if s.firstOut == 0 {
			s.firstOut = t
		}
		s.buf = append(s.buf, s.cur) // put it back: wasted work
		return
	}
	s.written++
}

// offer sends on a bounded channel. A plain send, ch <- v, waits while the
// channel is full: that is the "block" way. offer drops the oldest instead,
// only for data you can lose such as metrics.
func offer(ch chan int, v int) (dropped bool) {
	select {
	case ch <- v: // there is room
		return false
	default:
		<-ch // full: drop the oldest
		ch <- v
		return true
	}
}

func main() {
	fmt.Println("push with no limit:")
	s := &Sync{}
	for t := 0; ; t++ {
		s.tick(t)
		if t == slowFrom-1 {
			fmt.Printf("  tick %d: same speed, buffer %d page, written %d\n", t+1, len(s.buf), s.written)
		}
		if t == 29 {
			fmt.Printf("  tick %d: DozerDB slow since tick %d, buffer %d pages\n", t+1, slowFrom, len(s.buf))
		}
		if s.firstOut == t && t > 0 {
			fmt.Printf("  tick %d: first timeout, the page waited over %d ticks\n", t+1, timeout)
		}
		// nothing else stops the growth in the push run
		if len(s.buf) > memLimit {
			fmt.Printf("  tick %d: out of memory with %d pages waiting, %d retries\n", t+1, len(s.buf), s.retries)
			fmt.Printf("  read %d, written %d: %d pages to read again after the restart\n", s.read, s.written, s.read-s.written)
			break // out of memory: restart, read the pages again
		}
	}

	fmt.Println("pull, 2 pages at a time:")
	p := &Sync{pull: true, demand: pullSize}
	for t := 0; t < 90; t++ {
		p.tick(t)
		if t == slowFrom-1 || t == slowUntil-1 || t == 89 {
			fmt.Printf("  tick %d: written %d, reader idle %d ticks, max buffer %d pages, retries %d\n",
				t+1, p.written, p.idle, p.maxBuf, p.retries)
		}
	}

	fmt.Println("drop the oldest, a bounded channel of 2 metrics:")
	ch, lost := make(chan int, pullSize), 0
	for v := 1; v <= 5; v++ {
		if offer(ch, v) {
			lost++
		}
	}
	fmt.Printf("  sent 5, kept %d and %d, dropped %d\n", <-ch, <-ch, lost)
}
