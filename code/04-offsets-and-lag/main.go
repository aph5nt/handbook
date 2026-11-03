// Offsets and consumer lag, and where a group starts when it has no
// valid committed offset (auto.offset.reset).
//
// A Go struct stands in for one Kafka partition: a log start offset (the
// oldest record retention still keeps), the end offset (the next offset to
// be written) and each record's write time. Deterministic.
package main

import "fmt"

type Partition struct {
	start int   // log start offset: older records were deleted by retention
	times []int // write time in minutes of each offset, index = offset
}

func (p *Partition) End() int          { return len(p.times) }
func (p *Partition) Append(minute int) { p.times = append(p.times, minute) }

// Lag in messages: end offset minus committed offset.
func (p *Partition) Lag(committed int) int { return p.End() - committed }

// Lag in time: how old the oldest unread record is.
func (p *Partition) LagMinutes(committed, now int) int {
	if committed >= p.End() {
		return 0
	}
	return now - p.times[committed]
}

// startOffset is where a group begins reading.
func startOffset(p *Partition, committed int, has bool, reset string) int {
	if has && committed >= p.start && committed <= p.End() {
		return committed // a valid bookmark
	}
	if reset == "earliest" {
		return p.start // the oldest record still kept
	}
	return p.End() // latest: only new records
}

// process handles the record at one offset.
func process(off int) {
	fmt.Println("processed offset", off)
}

func main() {
	fmt.Println("== Tab 1: lag ==")
	p := &Partition{}
	for o := 0; o < 12; o++ {
		p.Append(o) // one record a minute
	}
	committed := 8
	fmt.Printf("step 1: end offset %d, committed %d, lag %d\n", p.End(), committed, p.Lag(committed))
	for committed < p.End() {
		process(committed)
		committed++ // commit
	}
	fmt.Printf("step 2: processed 8..11, committed %d, lag %d\n", committed, p.Lag(committed))
	for o := 12; o < 18; o++ {
		p.Append(12) // a busy block: six at once at minute 12
	}
	fmt.Printf("step 3: burst 12..17, end %d, lag %d\n", p.End(), p.Lag(committed))
	committed = 16
	fmt.Printf("step 4: committed %d, lag %d\n", committed, p.Lag(committed))
	// The consumer hangs on offset 16. Quiet night: nothing new for 3 hours.
	now := 12 + 60*3
	fmt.Printf("step 5: 3 hours later: lag %d messages (alert at 1,000: silent), oldest unread is %d minutes old\n",
		p.Lag(committed), p.LagMinutes(committed, now))

	fmt.Println("== Tab 2: reset ==")
	q := &Partition{}
	for o := 0; o < 12; o++ {
		q.Append(o)
	}
	fmt.Printf("step 2: new group, latest   -> starts at %d, skips 0..11\n", startOffset(q, 0, false, "latest"))
	fmt.Printf("step 3: new group, earliest -> starts at %d, replays 12 records\n", startOffset(q, 0, false, "earliest"))
	q.start = 6                                     // down 9 days, retention 7 days: offsets 0..5 deleted
	latest := startOffset(q, 4, true, "latest")     // 12: 4..11 skipped
	earliest := startOffset(q, 4, true, "earliest") // 6
	fmt.Printf("step 4: committed 4, log start now %d: latest -> %d (4..11 skipped)\n", q.start, latest)
	fmt.Printf("step 5: same with earliest -> %d (4 and 5 are gone, 6..11 are read)\n", earliest)
}
