package main

import (
	"fmt"
	"sort"
	"time"
)

const (
	outOfOrder      = 10 * time.Second // the watermark trails the newest swap by this
	allowedLateness = 60 * time.Second // a fired window keeps its state this long
	maxClockSkew    = 5 * time.Second  // an event time this far ahead of arrival is a bad clock
)

type Window struct {
	Count int
	Fired bool
}

type Counter struct {
	maxSeen    time.Time
	windows    map[time.Time]*Window // open or fired-but-kept windows, by start
	board      map[time.Time]int     // the sink, keyed by window start
	late       []time.Time           // side output: too late, fixed by the nightly batch
	quarantine []time.Time           // bad clocks, never move the watermark
}

func NewCounter() *Counter {
	return &Counter{windows: map[time.Time]*Window{}, board: map[time.Time]int{}}
}

func (c *Counter) OnSwap(eventTime, arrival time.Time) {
	if eventTime.After(arrival.Add(maxClockSkew)) {
		c.quarantine = append(c.quarantine, eventTime)
		return
	}
	if eventTime.After(c.maxSeen) {
		c.maxSeen = eventTime
	}
	watermark := c.maxSeen.Add(-outOfOrder)
	start := eventTime.Truncate(time.Minute)
	end := start.Add(time.Minute)

	if !watermark.Before(end.Add(allowedLateness)) {
		c.late = append(c.late, eventTime) // its state is gone
		return
	}
	w, ok := c.windows[start]
	if !ok {
		w = &Window{}
		c.windows[start] = w
	}
	w.Count++
	if w.Fired {
		c.board[start] = w.Count // late, inside the grace: overwrite the same key
	}
	for s, v := range c.windows {
		vEnd := s.Add(time.Minute)
		if !v.Fired && !watermark.Before(vEnd) {
			c.board[s] = v.Count // the on-time answer
			v.Fired = true
		}
		if !watermark.Before(vEnd.Add(allowedLateness)) {
			delete(c.windows, s) // grace over: free the memory
		}
	}
}

func (c *Counter) dump(label string) {
	keys := []time.Time{}
	for k := range c.board {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Before(keys[j]) })
	fmt.Printf("%-34s wm=%s board=", label, c.maxSeen.Add(-outOfOrder).Format("15:04:05"))
	for _, k := range keys {
		fmt.Printf("%s:%d ", k.Format("15:04"), c.board[k])
	}
	st := []string{}
	for k, v := range c.windows {
		st = append(st, fmt.Sprintf("%s=%d/%v", k.Format("15:04"), v.Count, v.Fired))
	}
	sort.Strings(st)
	fmt.Println("state", st, "late", len(c.late), "quar", len(c.quarantine))
}

// The batch path, shown in the lesson's batch tab. StarRocks SQL, printed
// here and not run: the stream path above is the part this program runs.
const appendSQL = `-- on every arrival: append, no counting
INSERT INTO swaps (event_time, day, pool) VALUES (?, ?, ?);
`

const nightlySQL = `-- every night at 00:05, for yesterday
INSERT OVERWRITE swaps_per_minute PARTITION (day = '2026-09-24')
SELECT date_trunc('minute', event_time) AS minute,
       count(*)                          AS swaps
FROM   swaps
WHERE  day = '2026-09-24'
GROUP  BY 1;
`

func main() {
	base := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	at := func(s int) time.Time { return base.Add(time.Duration(s) * time.Second) }
	c := NewCounter()
	for _, e := range [][2]int{{10, 11}, {30, 31}, {50, 52}, {75, 76}, {80, 81}, {100, 101}, {40, 105}, {130, 131}, {20, 140}} {
		c.OnSwap(at(e[0]), at(e[1]))
		c.dump(fmt.Sprintf("swap %s arrives %s", at(e[0]).Format("15:04:05"), at(e[1]).Format("15:04:05")))
	}
	fmt.Println("--- bad clock, guard off")
	c = NewCounter()
	for _, e := range [][2]int{{10, 11}, {30, 31}, {50, 52}, {175, 55}, {58, 59}, {65, 66}} {
		// guard off: pretend arrival equals event time
		c.OnSwap(at(e[0]), at(e[0]))
		c.dump(fmt.Sprintf("swap %s", at(e[0]).Format("15:04:05")))
	}
	fmt.Println("--- bad clock, guard on")
	c = NewCounter()
	for _, e := range [][2]int{{10, 11}, {30, 31}, {50, 52}, {175, 55}, {58, 59}, {65, 66}} {
		c.OnSwap(at(e[0]), at(e[1]))
		c.dump(fmt.Sprintf("swap %s arrives %s", at(e[0]).Format("15:04:05"), at(e[1]).Format("15:04:05")))
	}
	fmt.Println("--- batch path (StarRocks SQL, not run here)")
	fmt.Print(appendSQL)
	fmt.Print(nightlySQL)
}
