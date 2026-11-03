// Replays distributed tracing: a trace id in the traceparent header over four
// hops, spans joined by parent id, then one client that forgets the header.
package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// Span is one service's piece of work, in milliseconds from the first byte.
type Span struct {
	Trace, ID, Parent, Name string
	Start, End              int
}

// Inject writes the W3C traceparent header for the next hop.
func Inject(h http.Header, trace, span, flag string) {
	h.Set("traceparent", "00-"+trace+"-"+span+"-"+flag)
}

// Extract reads it. No header means no parent: this hop starts a new trace.
func Extract(h http.Header) (trace, parent string, ok bool) {
	p := strings.Split(h.Get("traceparent"), "-")
	if len(p) != 4 {
		return "", "", false
	}
	return p[1], p[2], true
}

// Open starts a span from the incoming headers.
func Open(h http.Header, id, name, newTrace string, start, end int) Span {
	trace, parent, ok := Extract(h)
	if !ok {
		trace, parent = newTrace, ""
	}
	return Span{trace, id, parent, name, start, end}
}

// NewClient is the price service's new HTTP client: no tracing wrapper, so no header.
func NewClient() http.Header {
	return http.Header{}
}

// LogLine is what each service writes while the span runs: the ids join log and trace.
func LogLine(s Span, text string) string {
	return fmt.Sprintf("trace_id=%s span_id=%s %q", s.Trace, s.ID, text)
}

// Collect groups spans by trace id, as the collector does.
func Collect(spans []Span) map[string][]Span {
	traces := map[string][]Span{}
	for _, s := range spans {
		traces[s.Trace] = append(traces[s.Trace], s)
	}
	return traces
}

// SelfTime is a span's time minus the time of its children.
func SelfTime(s Span, all []Span) int {
	self := s.End - s.Start
	for _, c := range all {
		if c.Parent == s.ID {
			self -= c.End - c.Start
		}
	}
	return self
}

// Slowest names the span where the time really went.
func Slowest(all []Span) (string, int) {
	name, most := "", -1
	for _, s := range all {
		if t := SelfTime(s, all); t > most {
			name, most = s.Name, t
		}
	}
	return name, most
}

// KeepTail is the collector's rule, run when the trace is complete.
func KeepTail(all []Span, failed bool) bool {
	for _, s := range all {
		if s.End-s.Start > 500 {
			return true
		}
	}
	return failed
}

// run walks the four hops. wrapped says whether the last client sends the header.
func run(wrapped bool) []Span {
	h := http.Header{}
	a := Open(h, "A", "proxy", "7f3a", 0, 1000)
	Inject(h, a.Trace, a.ID, "01")
	b := Open(h, "B", "graph API", "x", 40, 960)
	Inject(h, b.Trace, b.ID, "01")
	c := Open(h, "C", "price service", "x", 120, 920)
	if wrapped {
		Inject(h, c.Trace, c.ID, "01")
	} else {
		h = NewClient()
	}
	d := Open(h, "D", "StarRocks", "91c2", 160, 900)
	return []Span{a, b, c, d}
}

func show(spans []Span) {
	traces := Collect(spans)
	ids := make([]string, 0, len(traces))
	for id := range traces {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] > ids[j] })
	for _, id := range ids {
		name, self := Slowest(traces[id])
		fmt.Printf("  trace %s: %d spans, the time went to %s, %d ms of its own\n", id, len(traces[id]), name, self)
	}
}

func main() {
	fmt.Println("all hops wrapped:")
	spans := run(true)
	for _, s := range spans {
		fmt.Printf("  span %s %-14s trace %s parent %-2q %d ms\n", s.ID, s.Name, s.Trace, s.Parent, s.End-s.Start)
	}
	show(spans)
	fmt.Println("  log:", LogLine(spans[3], "scanned 40 partitions"))
	fmt.Println("one client not wrapped:")
	show(run(false))
	fmt.Println("tail rule keeps the 1,000 ms trace:", KeepTail(spans, false))
	fast := []Span{{"aa", "A", "", "proxy", 0, 90}}
	fmt.Println("tail rule keeps a 90 ms trace:", KeepTail(fast, false))
	h := http.Header{}
	Inject(h, "7f3a", "A", "00")
	fmt.Println("head sampling, the header says sampled=0:", h.Get("traceparent"))
}
