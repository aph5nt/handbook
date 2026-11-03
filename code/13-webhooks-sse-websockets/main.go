// A payment provider posts events to a subscription service. Replays the
// lesson 13-webhooks-sse-websockets: a naive receiver, a signed and
// deduplicated one, retries to a dead-letter list, then SSE (real, over
// HTTP) and WebSockets (modelled with channels: no standard-library client).
package main

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"
)

var secret = []byte("shared-with-the-provider")

type Event struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Account string `json:"account"`
}

// ---- the naive receiver: slow work first, no id, no signature

type Naive struct {
	months map[string]int
	work   time.Duration
}

func (n *Naive) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var e Event
	json.NewDecoder(r.Body).Decode(&e)
	time.Sleep(n.work) // unlock, email, invoice, all before the answer
	switch e.Type {
	case "payment.settled":
		n.months[e.Account]++
	case "payment.refunded":
		n.months[e.Account] = max(0, n.months[e.Account]-1)
	}
	w.WriteHeader(http.StatusOK)
}

// ---- the safe receiver: verify, store the id, answer, work later

type Safe struct {
	now    func() int64
	seen   map[string]bool
	queue  chan Event
	months map[string]int
}

func sign(ts string, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(ts + "." + string(body)))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *Safe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	ts := r.Header.Get("X-Timestamp")
	sent, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || s.now()-sent > 300 { // older than five minutes: a replay
		http.Error(w, "stale", http.StatusUnauthorized)
		return
	}
	if !hmac.Equal([]byte(sign(ts, body)), []byte(r.Header.Get("X-Signature"))) {
		http.Error(w, "bad signature", http.StatusUnauthorized)
		return
	}
	var e Event
	json.Unmarshal(body, &e)
	if s.seen[e.ID] {
		w.WriteHeader(http.StatusOK) // already stored: do nothing
		return
	}
	s.seen[e.ID] = true
	s.queue <- e // the unlock runs in the background
	w.WriteHeader(http.StatusOK)
}

func (s *Safe) worker() {
	for e := range s.queue {
		if e.Type == "payment.settled" {
			s.months[e.Account]++
		}
	}
}

// ---- the provider's side

func post(url string, e Event, ts int64, sig string, timeout time.Duration) (int, error) {
	body, _ := json.Marshal(e)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("X-Timestamp", strconv.FormatInt(ts, 10))
	if sig == "sign" {
		sig = sign(strconv.FormatInt(ts, 10), body)
	}
	req.Header.Set("X-Signature", sig)
	res, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return 0, err
	}
	res.Body.Close()
	return res.StatusCode, nil
}

// deliver retries with growing gaps, then moves the event to the dead list.
func deliver(try func() bool, gaps []int) (minutes int, delivered bool) {
	for _, gap := range gaps {
		if try() {
			return minutes, true
		}
		minutes += gap
	}
	return minutes, false
}

// ---- browsers: SSE is real HTTP, one way

func sse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	f := w.(http.Flusher)
	for h := 9506; h <= 9508; h++ {
		fmt.Fprintf(w, "data: block %d\n\n", h)
		f.Flush()
	}
}

// socket models a WebSocket: two channels, both sides send freely.
func socket(fromUser <-chan string, toUser chan<- string) {
	for cmd := range fromUser {
		toUser <- "agent ran: " + cmd
	}
	close(toUser)
}

func main() {
	fmt.Println("--- naive receiver")
	n := &Naive{months: map[string]int{}, work: 60 * time.Millisecond}
	srv := httptest.NewServer(n)
	settled := Event{"e9", "payment.settled", "alice"}
	_, err := post(srv.URL, settled, 0, "", 20*time.Millisecond)
	fmt.Println("first delivery: provider gave up:", err != nil)
	time.Sleep(80 * time.Millisecond)
	post(srv.URL, settled, 0, "", time.Second)
	fmt.Println("after the retry: alice has months", n.months["alice"], "for one payment")
	post(srv.URL, Event{"x1", "payment.settled", "mallory"}, 0, "", time.Second)
	fmt.Println("fake event: mallory has months", n.months["mallory"])
	post(srv.URL, Event{"r1", "payment.refunded", "bob"}, 0, "", time.Second)
	post(srv.URL, Event{"s1", "payment.settled", "bob"}, 0, "", time.Second)
	fmt.Println("refund before settle: refunded bob has months", n.months["bob"])
	srv.Close()

	fmt.Println("--- signed, deduplicated, fast")
	now := int64(1_800_000_000)
	s := &Safe{now: func() int64 { return now }, seen: map[string]bool{}, queue: make(chan Event, 8), months: map[string]int{}}
	go s.worker()
	srv = httptest.NewServer(s)
	defer srv.Close()
	code, _ := post(srv.URL, settled, now, "sign", time.Second)
	fmt.Println("signed e9:", code)
	code, _ = post(srv.URL, settled, now, "sign", time.Second)
	fmt.Println("e9 again:", code)
	code, _ = post(srv.URL, Event{"x1", "payment.settled", "mallory"}, now, "deadbeef", time.Second)
	fmt.Println("fake:", code)
	code, _ = post(srv.URL, settled, now-301, "sign", time.Second)
	fmt.Println("captured and replayed 301 s later:", code)
	close(s.queue)
	time.Sleep(10 * time.Millisecond)
	fmt.Println("alice months", s.months["alice"], "mallory months", s.months["mallory"])

	fails := 0
	minutes, ok := deliver(func() bool { fails++; return false }, []int{1, 5, 15, 30, 60})
	fmt.Printf("receiver down: %d tries over %d minutes, delivered %v -> dead-letter list, replay later\n", fails, minutes, ok)

	fmt.Println("--- browsers")
	feed := httptest.NewServer(http.HandlerFunc(sse))
	defer feed.Close()
	res, err := http.Get(feed.URL)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	sc := bufio.NewScanner(res.Body)
	for sc.Scan() {
		if line, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
			fmt.Println("SSE:", line)
		}
	}
	up, down := make(chan string), make(chan string)
	go socket(up, down)
	up <- "trace 0xab"
	fmt.Println("WebSocket:", <-down)
	close(up)
}
