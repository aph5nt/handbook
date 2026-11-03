// A thousand agents retry against an API that went down. Replays the lesson
// 13-backoff-with-jitter: doubling alone keeps them in step, full jitter
// spreads them, Retry-After wins, and retries multiply across layers.
// Waits are computed, not slept, so the numbers are the same on every run.
package main

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
)

const (
	base     = 1.0  // first window, seconds
	maxWait  = 30.0 // cap on the window
	maxTries = 5
)

// window doubles with each try, up to the cap.
func window(try int) float64 {
	w := base
	for i := 0; i < try; i++ {
		w *= 2
	}
	return min(w, maxWait)
}

// plain waits the whole window: every agent computes the same number.
func plain(try int, _ *rand.Rand) float64 { return window(try) }

// jitter waits a random time between zero and the window.
func jitter(try int, rng *rand.Rand) float64 { return rng.Float64() * window(try) }

// arrivals counts retries per 100 ms slot over three rounds, for n agents.
func arrivals(n int, wait func(int, *rand.Rand) float64) map[int]map[int]int {
	rng := rand.New(rand.NewSource(1))
	rounds := map[int]map[int]int{}
	for a := 0; a < n; a++ {
		at := 0.0
		for try := 0; try < 3; try++ {
			at += wait(try, rng)
			if rounds[try] == nil {
				rounds[try] = map[int]int{}
			}
			rounds[try][int(at*10)]++
		}
	}
	return rounds
}

func busiest(slots map[int]int) int {
	top := 0
	for _, n := range slots {
		top = max(top, n)
	}
	return top
}

var errGaveUp = errors.New("gave up after the set number of tries")

// retry calls the API with full jitter, honours Retry-After and stops.
func retry(url string, rng *rand.Rand) (tries int, waited float64, err error) {
	for try := 0; try < maxTries; try++ {
		tries++
		res, err := http.Get(url)
		if err != nil {
			return tries, waited, err
		}
		res.Body.Close()
		if res.StatusCode < 500 && res.StatusCode != http.StatusTooManyRequests {
			return tries, waited, nil
		}
		wait := jitter(try, rng)
		if s, e := strconv.Atoi(res.Header.Get("Retry-After")); e == nil {
			wait = max(wait, float64(s)) // at least what the server said
		}
		waited += wait
	}
	return tries, waited, errGaveUp
}

// server fails the first n calls, each with an optional Retry-After.
func server(n int, retryAfter string) *httptest.Server {
	calls := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls > n {
			return
		}
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
}

func main() {
	const agents = 1000
	fmt.Println("--- backoff, no jitter:", agents, "agents")
	lock := arrivals(agents, plain)
	for try := 0; try < 3; try++ {
		fmt.Println("round", try+1, "busiest 100 ms slot:", busiest(lock[try]))
	}
	layers, each := 3, 3
	calls := 1
	for i := 0; i < layers; i++ {
		calls *= each
	}
	fmt.Println("app, SDK and gateway each try", each, "times: one user action, up to", calls, "calls")

	fmt.Println("--- backoff with full jitter")
	spread := arrivals(agents, jitter)
	for try := 0; try < 3; try++ {
		fmt.Println("round", try+1, "busiest 100 ms slot:", busiest(spread[try]))
	}
	rng := rand.New(rand.NewSource(1))
	fmt.Printf("agent waits: %.1f %.1f %.1f s, inside a 1 s window\n", jitter(0, rng), jitter(0, rng), jitter(0, rng))

	s := server(1, "5")
	defer s.Close()
	tries, waited, err := retry(s.URL, rng)
	fmt.Printf("server says Retry-After 5: tries %d, waited %.0f s, err %v\n", tries, waited, err)
	down := server(100, "")
	defer down.Close()
	tries, _, err = retry(down.URL, rng)
	fmt.Println("server stays down: tries", tries, "|", err)
}
