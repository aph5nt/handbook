// Three signals on one slow query. The metric says when, the trace says
// where, the log says why. Then the cardinality trap: series multiply with
// every label value.
package main

import (
	"fmt"
	"strings"
)

// Span is one hop of a trace, in seconds.
type Span struct {
	Name     string
	Seconds  float64
	Children []*Span
	Attrs    map[string]string // ids go here: an address, a transaction hash
}

// slowHop finds the span whose own time, without its children, is largest.
func slowHop(s *Span) (name string, own float64) {
	own = s.Seconds
	name = s.Name
	best := 0.0
	bestName := ""
	for _, c := range s.Children {
		own -= c.Seconds
		if n, o := slowHop(c); o > best {
			bestName, best = n, o
		}
	}
	if best > own {
		return bestName, best
	}
	return name, own
}

// Series counts one time series per distinct label combination.
type Series map[string]bool

func (s Series) Observe(labels ...string) { s[strings.Join(labels, ",")] = true }

// grep is the log query: filter by a label, then search the text.
func grep(logs []string, label, text string) []string {
	var out []string
	for _, l := range logs {
		if strings.Contains(l, label) && strings.Contains(l, text) {
			out = append(out, l)
		}
	}
	return out
}

// exporters is configuration: where each signal goes. The code never changes.
var exporters = map[string]string{"metrics": "prometheus", "traces": "tempo", "logs": "loki"}

const bytesPerSeries = 3000 // an order of magnitude, a few KB each

func main() {
	latency := map[string]float64{"13:50": 0.4, "13:55": 0.4, "14:00": 28, "14:05": 25}
	for _, t := range []string{"13:50", "13:55", "14:00", "14:05"} {
		fmt.Printf("metric p95 at %s: %.1f s\n", t, latency[t])
	}

	db := &Span{Name: "database call", Seconds: 29}
	graph := &Span{Name: "graph query", Seconds: 29.8, Children: []*Span{db}}
	api := &Span{Name: "API", Seconds: 30, Children: []*Span{graph}}
	wallet := "0x18a3f1d6e0b2c4a7958e3d1f6b0c2a4e7d9f1b3c"
	api.Attrs = map[string]string{"address": wallet} // the id on the span, not a label
	name, own := slowHop(api)
	fmt.Printf("trace: slowest hop is %s, %.0f s of its own\n", name, own)

	logs := []string{
		"14:00:02 service=graph-db page cache restarted",
		"14:00:03 service=graph-db cold reads from disk",
		"14:00:04 service=api request done",
	}
	for _, l := range grep(logs, "service=graph-db", "") {
		fmt.Println("log:", l)
	}

	small := Series{}
	for _, status := range []string{"ok", "error", "timeout"} {
		small.Observe("graph", status) // labels: small fixed sets
	}
	fmt.Printf("labels service, status: %d series\n", len(small))
	big := Series{}
	for i := 0; i < 2000000; i++ {
		big.Observe("graph", "ok", fmt.Sprintf("0x%040x", i)) // an address as a label
	}
	fmt.Printf("add label address, 2,000,000 wallets: %d series, about %d GB of memory\n",
		len(big), len(big)*bytesPerSeries/1000000000)
	fmt.Printf("the address stays findable on the span: %s\n", api.Attrs["address"])
	fmt.Println("where each signal goes is configuration:", exporters["metrics"], exporters["traces"], exporters["logs"])
}
