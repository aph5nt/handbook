// Command hystrix replays the Hystrix lesson: a circuit breaker with a
// rolling window, a bulkhead pool, a half-open test call, a threshold that
// aged, and the adaptive limit that replaced fixed numbers.
// Time is simulated, so the run is instant and the numbers are exact.
package main

import (
	"fmt"
	"time"
)

type State string

const (
	Closed   State = "CLOSED"
	Open     State = "OPEN"
	HalfOpen State = "HALF_OPEN"
)

type Breaker struct {
	State    State
	OpenedAt time.Duration
	Window   []bool // last 20 results, true = ok
	InUse    int    // threads busy in this dependency's pool
	Timeout  time.Duration
}

const (
	poolSize    = 10
	windowSize  = 20
	sleepWindow = 5 * time.Second
)

// Call runs one request that takes latency, at simulated time now.
func (b *Breaker) Call(now, latency time.Duration) (ok bool, path string) {
	switch b.State {
	case Open:
		if now-b.OpenedAt < sleepWindow {
			return false, "fallback (open, fail fast)"
		}
		b.State = HalfOpen
	case HalfOpen:
		return false, "fallback (test in flight)"
	}
	if b.InUse == poolSize {
		ok, path = false, "fallback (bulkhead full)"
	} else {
		ok, path = latency <= b.Timeout, "answer"
	}
	b.Window = append(b.Window, ok)
	if len(b.Window) > windowSize {
		b.Window = b.Window[1:]
	}
	if b.State == HalfOpen {
		if ok {
			b.State, b.Window = Closed, nil
		} else {
			b.State, b.OpenedAt = Open, now
		}
	} else if len(b.Window) >= windowSize && b.failRate() >= 0.5 {
		b.State, b.OpenedAt = Open, now
	}
	if !ok && path == "answer" {
		path = "fallback (timeout)"
	}
	return ok, path
}

// AdaptiveLimit is the gradient rule from Netflix concurrency-limits.
func AdaptiveLimit(limit float64, minRTT, sampleRTT time.Duration) float64 {
	return limit * float64(minRTT) / float64(sampleRTT)
}

func (b *Breaker) failRate() float64 {
	bad := 0
	for _, ok := range b.Window {
		if !ok {
			bad++
		}
	}
	return float64(bad) / float64(len(b.Window))
}

func (b *Breaker) fails() int { return int(b.failRate()*float64(len(b.Window)) + 0.5) }

func main() {
	ms := time.Millisecond
	b := &Breaker{State: Closed, Timeout: 300 * ms}
	t := time.Duration(0)

	fmt.Println("step 1: 20 calls at 50 ms, timeout 300 ms")
	for i := 0; i < 20; i++ {
		b.Call(t, 50*ms)
		t += 100 * ms
	}
	fmt.Printf("  state %s, failures %d of %d\n", b.State, b.fails(), len(b.Window))

	fmt.Println("step 2: Recommendations answers in 5 s; 6 calls time out")
	for i := 0; i < 6; i++ {
		_, p := b.Call(t, 5*time.Second)
		if i == 0 {
			fmt.Println("  first slow call:", p)
		}
		t += 100 * ms
	}
	fmt.Printf("  state %s, failures %d of %d, rate %.0f%% < 50%%: stay closed\n", b.State, b.fails(), len(b.Window), b.failRate()*100)

	fmt.Println("step 3: 10 threads stuck in the pool, new calls are refused")
	b.InUse = poolSize
	for i := 0; i < 20; i++ {
		_, p := b.Call(t, 5*time.Second)
		if b.State == Open {
			fmt.Printf("  refused call %d: %s\n", i+1, p)
			fmt.Printf("step 4: failures %d of %d = %.0f%% >= 50%%: state OPEN\n", b.fails(), len(b.Window), b.failRate()*100)
			break
		}
		t += 100 * ms
	}
	b.InUse = 0

	fmt.Println("  the next call fails fast")
	_, p := b.Call(t+1*time.Second, 50*ms)
	fmt.Println("  1 s after opening:", p)

	fmt.Println("step 5: 5 s after opening, one test call goes through")
	ok, p := b.Call(b.OpenedAt+sleepWindow, 50*ms)
	fmt.Println("  test call:", p, "ok =", ok)
	fmt.Println("step 6: state", b.State, "window cleared:", len(b.Window) == 0)
	t = b.OpenedAt + 10*time.Second

	fmt.Println("step 7: a year later, Friday latency 350 ms, timeout still 300 ms")
	for i := 0; i < 20 && b.State == Closed; i++ {
		b.Call(t, 350*ms)
		t += 100 * ms
	}
	fmt.Printf("  healthy service, state %s after %d calls: the threshold aged\n", b.State, len(b.Window))

	fmt.Println("step 8: adaptive limit, no fixed timeout")
	fmt.Printf("  limit 40, minRTT 50 ms, sample 62.5 ms -> %.0f in flight\n",
		AdaptiveLimit(40, 50*ms, 62500*time.Microsecond))
}
