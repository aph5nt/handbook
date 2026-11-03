// Retention and tiered storage: what a broker deletes, and when.
//
// A slice of daily segments stands in for one partition. A segment goes when
// it is older than retention.ms, or when the partition is over retention.bytes,
// whichever comes first. Sizes are this example's: a normal day is 10 GB.
// Deterministic.
package main

import "fmt"

type Segment struct{ Day, GB int }

// retain applies both limits and returns what stays.
func retain(segs []Segment, today, maxDays, maxGB int) []Segment {
	var kept []Segment
	for _, s := range segs {
		if maxDays > 0 && today-s.Day >= maxDays {
			continue // older than retention.ms
		}
		kept = append(kept, s)
	}
	total := 0
	for _, s := range kept {
		total += s.GB
	}
	for maxGB > 0 && total > maxGB {
		total -= kept[0].GB // oldest first
		kept = kept[1:]
	}
	return kept
}

func days(segs []Segment) string {
	if len(segs) == 0 {
		return "none"
	}
	return fmt.Sprintf("day %d to day %d", segs[0].Day, segs[len(segs)-1].Day)
}

func oldestDay(segs []Segment) int { return segs[0].Day }

// readForward: the stored offset still exists, the consumer goes on from it.
func readForward(name string) {
	fmt.Printf("%s: offset is kept, it reads forward\n", name)
}

// autoOffsetReset: the stored offset was deleted, the setting picks the start.
func autoOffsetReset(name string) {
	fmt.Printf("%s: offset is deleted, auto.offset.reset decides\n", name)
}

func main() {
	fmt.Println("== Tab 1: time ==")
	var segs []Segment
	for d := 1; d <= 7; d++ {
		segs = append(segs, Segment{d, 10})
	}
	fmt.Println("retention 7 days, today is day 7, kept:", days(retain(segs, 7, 7, 0)))
	segs = append(segs, Segment{8, 10})
	kept := retain(segs, 8, 7, 0)
	fmt.Println("day 8 arrives, kept:", days(kept))
	for _, c := range []struct {
		name string
		off  int
	}{{"graph sync, down 3 days, offset on day 5", 5}, {"second consumer, down 9 days, offset on day 0", 0}} {
		// consumer on restart
		if c.off >= oldestDay(kept) {
			readForward(c.name)
		} else {
			autoOffsetReset(c.name)
		}
	}

	fmt.Println("== Tab 2: size ==")
	const limit = 75 // retention.bytes, in GB
	segs = nil
	for d := 1; d <= 7; d++ {
		segs = append(segs, Segment{d, 10})
	}
	fmt.Printf("normal week: %d GB under %d GB, kept %s\n", 70, limit, days(retain(segs, 7, 7, limit)))
	segs = append(segs, Segment{8, 30}, Segment{9, 30}) // a launch: 3 times the traffic
	kept = retain(segs, 9, 7, limit)
	fmt.Printf("launch, two days at 30 GB: kept %s, only %d days\n", days(kept), len(kept))
	promisedDay := 3 // 7 days back from day 9
	alert := oldestDay(kept) > promisedDay
	fmt.Printf("oldest kept is day %d. Promised 7 days back from day 9 is day %d: alert=%v\n", oldestDay(kept), promisedDay, alert)

	fmt.Println("== Tab 3: tiered ==")
	const localDays, remoteDays = 1, 90
	read := func(age int) string {
		switch {
		case age < localDays:
			return "broker disk"
		case age < remoteDays:
			return "object storage"
		}
		return "deleted"
	}
	for _, age := range []int{0, 30, 95} { // broker disk, object storage, deleted
		fmt.Printf("a record %d days old is served from: %s\n", age, read(age))
	}
	// five teams replaying 90 days at once: cap the remote fetches
	remoteFetchLimit := 2 // at a time, so live readers keep the network
	fmt.Printf("replays fetch %d segments from object storage at a time, in this example\n", remoteFetchLimit)
}
