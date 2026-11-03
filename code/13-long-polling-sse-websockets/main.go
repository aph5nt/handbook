// Three ways to push new blocks to a page. Replays the lesson
// 13-long-polling-sse-websockets. Long polling and SSE are real net/http.
// The WebSocket is modelled with a struct and a fake clock, because the
// standard library has no WebSocket. One story second is one millisecond.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"
)

const second = time.Millisecond

// Feed holds the blocks and wakes everyone waiting for the next one.
type Feed struct {
	mu     sync.Mutex
	latest int
	wake   chan struct{}
}

func (f *Feed) add(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.latest = n
	close(f.wake)
	f.wake = make(chan struct{})
}

// longPoll holds the request until a newer block exists, or until its timeout.
func (f *Feed) longPoll(w http.ResponseWriter, r *http.Request) {
	after, _ := strconv.Atoi(r.URL.Query().Get("after"))
	timeout := time.After(30 * second)
	for {
		f.mu.Lock()
		latest, wake := f.latest, f.wake
		f.mu.Unlock()
		if latest > after {
			json.NewEncoder(w).Encode(map[string]int{"block": latest})
			return
		}
		select {
		case <-wake:
		case <-timeout:
			w.WriteHeader(http.StatusNoContent) // nothing new: ask again
			return
		case <-r.Context().Done():
			return
		}
	}
}

// stream writes each block after Last-Event-ID, then ends: the drop.
func stream(upTo *int, ids bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		from := 102
		if last, err := strconv.Atoi(r.Header.Get("Last-Event-ID")); err == nil {
			from = last + 1
		} else if !ids {
			from = *upTo // a feed with no ids: only what is new from now on
		}
		for n := from; n <= *upTo; n++ {
			if ids {
				fmt.Fprintf(w, "id: %d\n", n)
			}
			fmt.Fprintf(w, "data: block %d\n\n", n)
			w.(http.Flusher).Flush()
		}
	}
}

// readEvents is the browser's EventSource: it remembers the last id it saw.
func readEvents(url, lastID string) (blocks []string, last string) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "text/event-stream")
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	last = lastID
	sc := bufio.NewScanner(res.Body)
	for sc.Scan() {
		if id, ok := strings.CutPrefix(sc.Text(), "id: "); ok {
			last = id
		}
		if data, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
			blocks = append(blocks, data)
		}
	}
	return blocks, last
}

func poll(url string) (int, int) {
	res, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	var b struct{ Block int }
	json.NewDecoder(res.Body).Decode(&b)
	return res.StatusCode, b.Block
}

// Socket models one WebSocket: two-way messages, a subscription, sequence
// numbers, and a proxy that cuts it after 60 quiet seconds.
type Socket struct {
	pool     int
	lastSeen int
}

const proxyIdle, pingEvery = 60, 20

// survives says whether the socket lives through a quiet gap.
func survives(gap int, heartbeat bool) bool {
	if heartbeat {
		return pingEvery < proxyIdle // each ping resets the proxy's idle clock
	}
	return gap < proxyIdle
}

// replay sends swaps the page missed, from its last sequence number.
func replay(log []int, after int) []int {
	var out []int
	for _, seq := range log {
		if seq > after {
			out = append(out, seq)
		}
	}
	return out
}

func main() {
	fmt.Println("--- long polling")
	f := &Feed{latest: 100, wake: make(chan struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /blocks", f.longPoll)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	go func() { time.Sleep(5 * second); f.add(101) }()
	code, block := poll(srv.URL + "/blocks?after=100")
	fmt.Println("held, then block lands:", code, block)
	code, _ = poll(srv.URL + "/blocks?after=101")
	fmt.Println("quiet for 30 s:", code, "(empty, ask again)")

	fmt.Println("--- SSE")
	upTo := 103
	sse := httptest.NewServer(stream(&upTo, true))
	defer sse.Close()
	blocks, last := readEvents(sse.URL, "")
	fmt.Println("stream:", blocks, "last id", last)
	upTo = 104 // written while nobody is connected
	blocks, last = readEvents(sse.URL, last)
	fmt.Println("reconnect with Last-Event-ID:", blocks, "last id", last)
	upTo = 103
	plain := httptest.NewServer(stream(&upTo, false))
	defer plain.Close()
	readEvents(plain.URL, "")
	upTo = 107
	blocks, _ = readEvents(plain.URL, "")
	fmt.Println("no ids, reconnect after 104 to 106:", blocks, "(the gap is lost)")

	fmt.Println("--- WebSocket (modelled)")
	s := &Socket{pool: 42}
	fmt.Println("subscribe pool", s.pool, "| page sends and server pushes on one connection")
	fmt.Println("quiet 90 s, no heartbeat, socket lives:", survives(90, false))
	fmt.Println("quiet 90 s, ping every 20 s, socket lives:", survives(90, true))
	s.lastSeen = 3
	fmt.Println("resume from sequence 3:", replay([]int{1, 2, 3, 4, 5}, s.lastSeen))
}
