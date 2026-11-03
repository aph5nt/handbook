// SSE (Server-Sent Events) resumption: events carry ids, and a reconnect with
// Last-Event-ID replays what was missed. No network: httptest. Run: go run .
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
)

type Event struct {
	ID   int
	Data string
}

// Stream keeps every event it sent, so it can replay.
type Stream struct {
	events  []Event
	withIDs bool
}

func (s *Stream) write(w http.ResponseWriter, e Event) {
	if s.withIDs {
		fmt.Fprintf(w, "id: %d\n", e.ID)
	}
	fmt.Fprintf(w, "data: %s\n\n", e.Data)
}

func (s *Stream) handler(dropAfter int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		last := -1
		if h := r.Header.Get("Last-Event-ID"); h != "" {
			last, _ = strconv.Atoi(h)
		} else if r.Method == "GET" {
			return // no position: nothing can be replayed
		}
		for _, e := range s.events {
			if e.ID <= last {
				continue // the client already has it
			}
			if r.Method == "POST" && e.ID > dropAfter {
				return // the Wi-Fi drops here
			}
			s.write(w, e)
		}
	}
}

// client reads a stream and remembers the last id it saw.
func read(body string) (lastID string, lines []string) {
	for _, l := range strings.Split(body, "\n") {
		if v, ok := strings.CutPrefix(l, "id: "); ok {
			lastID = v
		}
		if v, ok := strings.CutPrefix(l, "data: "); ok {
			lines = append(lines, v)
		}
	}
	return lastID, lines
}

func run(withIDs bool) {
	s := &Stream{withIDs: withIDs, events: []Event{{40, "progress 40%"}, {41, "progress 80%"}, {42, `{"id":8,"result":"rows"}`}}}
	h := s.handler(41)
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest("POST", "/mcp", nil))
	last, got := read(w.Body.String())
	fmt.Printf("  POST: %s, got %q, last id %q\n", w.Header().Get("Content-Type"), got, last)

	req := httptest.NewRequest("GET", "/mcp", nil)
	if last != "" {
		req.Header.Set("Last-Event-ID", last)
	}
	w = httptest.NewRecorder()
	h(w, req)
	_, got = read(w.Body.String())
	fmt.Printf("  GET Last-Event-ID %q: got %q\n", last, got)
}

func main() {
	fmt.Println("events with ids:")
	run(true)
	fmt.Println("events without ids:")
	run(false)
}
