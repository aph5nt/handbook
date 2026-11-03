// The indexer's metrics door in the Prometheus text format, and a rule
// evaluator. It replays the alert that pages during a re-index, the silence
// that swallows the real stall, and the better rule.
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
)

type Indexer struct {
	BlocksWritten float64
	BlocksBehind  float64
}

// metrics is the handler Prometheus scrapes.
func (ix *Indexer) metrics(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "# TYPE blocks_written_total counter\nblocks_written_total %.0f\n", ix.BlocksWritten)
	fmt.Fprintf(w, "# TYPE blocks_behind_head gauge\nblocks_behind_head %.0f\n", ix.BlocksBehind)
}

// logPage writes one line to stdout. An agent ships stdout to Loki.
func logPage(from, to int) { fmt.Printf("level=info msg=page_written from=%d to=%d\n", from, to) }

// Sample is one scrape: minutes since the start, and two numbers.
type Sample struct{ Min, Written, Behind float64 }

// behindFor is the old rule: behind by more than 1000 for 10 minutes.
func behindFor(h []Sample) bool {
	last := h[len(h)-1]
	for _, s := range h {
		if s.Min >= last.Min-10 && s.Behind <= 1000 {
			return false
		}
	}
	return last.Min >= 10
}

// stalled is the new rule: no new block written for 15 minutes.
func stalled(h []Sample) bool {
	last := h[len(h)-1]
	for _, s := range h {
		if s.Min >= last.Min-15 && s.Written < last.Written {
			return false
		}
	}
	return last.Min >= 15
}

// Silence hides a rule until End. End 0 means forever.
type Silence struct{ End float64 }

func notify(firing bool, now float64, s *Silence) string {
	switch {
	case !firing:
		return "quiet"
	case s != nil && (s.End == 0 || now < s.End):
		return "firing, silenced: nobody is told"
	}
	return "PAGE"
}

// route sends a firing alert by its tier.
func route(tier string) string {
	switch tier {
	case "page":
		return "PAGE: wake the on-call engineer"
	case "digest":
		return "add to the morning digest"
	}
	return "board only: send nothing"
}

func tier(name string) string {
	return map[string]string{"stopped": "page", "slower": "digest", "dip": "board only"}[name]
}

func main() {
	ix := &Indexer{BlocksWritten: 400000, BlocksBehind: 9000000}
	rec := httptest.NewRecorder()
	ix.metrics(rec, httptest.NewRequest("GET", "/metrics", nil))
	fmt.Printf("GET /metrics:\n%s", rec.Body.String())
	logPage(9506000, 9506199)

	var catchUp []Sample
	for m := 0.0; m <= 30; m++ {
		catchUp = append(catchUp, Sample{m, m * 1000, 9000000 - m*1000})
	}
	fmt.Println("re-index, healthy and busy:")
	fmt.Println("  old rule:", notify(behindFor(catchUp), 30, nil))
	fmt.Println("  new rule:", notify(stalled(catchUp), 30, nil))

	var stall []Sample
	for m := 0.0; m <= 30; m++ {
		w := min(m, 10) * 1000 // the node connection hangs at minute 10
		stall = append(stall, Sample{m, w, 9000000 - w})
	}
	fmt.Println("a real stall, weeks later:")
	fmt.Println("  old rule, silence with no end:", notify(behindFor(stall), 30, &Silence{}))
	fmt.Println("  new rule, no silence:         ", notify(stalled(stall), 30, nil))
	fmt.Println("  new rule, silence ended:      ", notify(stalled(stall), 30, &Silence{End: 20}))

	for _, a := range []string{"stopped", "slower", "dip"} {
		fmt.Printf("alert %s, tier %s: %s\n", a, tier(a), route(tier(a)))
	}
}
