// Replays the three pillars: one slow query recorded as a metric, a trace and
// a log line, joined by one trace id, and then the same query with nothing joined.
package main

import (
	"fmt"
	"hash/fnv"
)

// Stores holds what one proxy writes.
type Stores struct {
	Buckets   map[int]int    // upper bound in seconds -> count
	Exemplars map[int]string // bucket -> one trace id seen there
	Spans     map[string][]Span
	Logs      []Log
}

// Span is one piece of work on the request.
type Span struct {
	Name    string
	Seconds float64
}

// Log is one event with its detail.
type Log struct {
	TraceID, Minute, Text string
}

// RecordMetric adds to one histogram bucket and keeps the id as an exemplar.
func (s *Stores) RecordMetric(seconds float64, traceID string) {
	for _, upper := range []int{1, 2, 4, 8} {
		if seconds <= float64(upper) || upper == 8 {
			s.Buckets[upper]++
			if traceID != "" {
				s.Exemplars[upper] = traceID
			}
			return
		}
	}
}

// Handle is one request: all three pillars from one trace id.
func (s *Stores) Handle(traceID, minute string, keepTrace bool) {
	graph := Span{"graph", 4.0}
	proxy := Span{"proxy", 4.1}
	s.RecordMetric(proxy.Seconds, traceID)
	if keepTrace {
		s.Spans[traceID] = []Span{proxy, graph}
	}
	s.Logs = append(s.Logs, Log{traceID, minute, "plan scanned all addresses"})
}

// Follow is the on-call: chart to exemplar, exemplar to trace, trace to log.
func (s *Stores) Follow(bucket int) {
	id, ok := s.Exemplars[bucket]
	if !ok {
		fmt.Println("  chart: spike, but which request? no exemplar")
		return
	}
	fmt.Println("  1. chart spike, exemplar", id)
	for _, sp := range s.Spans[id] {
		fmt.Printf("  2. trace %s: %s %.1f s\n", id, sp.Name, sp.Seconds)
	}
	for _, l := range s.Logs {
		if l.TraceID == id {
			fmt.Println("  3. log:", l.Text)
		}
	}
}

// SearchByTime is all that is left without an id.
func SearchByTime(logs []Log, minute string) int {
	n := 0
	for _, l := range logs {
		if l.Minute == minute {
			n++
		}
	}
	return n
}

// KeepHead decides at the start of the request: 1 in 100, blind to how it ends.
func KeepHead(traceID string) bool {
	h := fnv.New32a()
	h.Write([]byte(traceID))
	return h.Sum32()%100 == 0
}

// KeepTail decides at the end: keep the slow and the failed.
func KeepTail(seconds float64, failed bool) bool {
	return seconds > 1 || failed
}

func newStores() *Stores {
	return &Stores{map[int]int{}, map[int]string{}, map[string][]Span{}, nil}
}

func main() {
	s := newStores()
	s.Handle("7f3a", "14:02", true)
	fmt.Println("tab 1: 4.1 s lands in bucket 8:", s.Buckets[8], "sample, exemplar", s.Exemplars[8])
	s.Follow(8)

	fmt.Println("\ntab 2: nothing joined")
	a := newStores()
	a.RecordMetric(4.1, "")
	a.Follow(8)
	for k := 0; k < 3000; k++ {
		a.Logs = append(a.Logs, Log{"", "14:02", "line"})
	}
	fmt.Println("  logs at 14:02:", SearchByTime(a.Logs, "14:02"), "lines. which one?")
	fmt.Println("  head sampling keeps 7f3a:", KeepHead("7f3a"))
	fmt.Println("  tail sampling keeps a 4.1 s request:", KeepTail(4.1, false))
}
