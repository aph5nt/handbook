// Command zuul replays the Zuul lesson: an inbound filter chain, a routing
// filter with a 1 % canary and region failover, outbound metrics, the thread
// exhaustion of blocking Zuul 1 and the per-route limits of Zuul 2.
package main

import (
	"fmt"
	"time"
)

type Request struct {
	User, Token string
	Device      int
	PerSecond   int // requests this device sent in the last second
}

type Gateway struct {
	Drained  map[string]bool // region -> evacuated
	InFlight map[string]int  // route -> calls waiting
	Metrics  map[string]int  // route -> answers
	Latency  map[string]time.Duration
}

type Filter struct {
	Name   string
	Status int
	Pass   func(Request) bool
}

var inbound = []Filter{
	{"auth", 401, func(r Request) bool { return r.Token == "valid" }},
	{"rate", 429, func(r Request) bool { return r.PerSecond <= 10 }},
}

func (g *Gateway) Handle(r Request, home, other string) (int, string) {
	for _, f := range inbound {
		if !f.Pass(r) {
			return f.Status, "stopped by " + f.Name
		}
	}
	route := "playback-api@" + home
	if g.Drained[home] {
		route = "playback-api@" + other
	}
	if r.Device%100 < 1 {
		route = "playback-canary@" + home
	}
	if g.InFlight[route] >= 100 {
		return 503, "shed: " + route
	}
	if g.Latency[route] > time.Second {
		return 504, "timeout after 1 s: " + route
	}
	g.Metrics[route]++
	return 200, route
}

// ThreadsNeeded is Little's law for blocking Zuul 1: each call holds a thread.
func ThreadsNeeded(perSecond int, latency time.Duration) int {
	return int(float64(perSecond) * latency.Seconds())
}

func main() {
	g := &Gateway{Drained: map[string]bool{}, InFlight: map[string]int{},
		Metrics: map[string]int{}, Latency: map[string]time.Duration{}}
	ola := Request{User: "Ola", Token: "valid", Device: 4711, PerSecond: 3}
	fmt.Println("step 1-2: Ola, device 4711, 3 requests per second")
	fmt.Println("  auth ok, rate 3 <= 10 ok")
	code, route := g.Handle(ola, "us-east", "us-west")
	fmt.Printf("step 3: 4711 %% 100 = %d, not < 1 -> %d %s\n", ola.Device%100, code, route)
	ben := Request{User: "Ben", Token: "valid", Device: 9300, PerSecond: 2}
	code, route = g.Handle(ben, "us-east", "us-west")
	fmt.Printf("step 4: Ben, device 9300 %% 100 = %d < 1 -> %d %s\n", ben.Device%100, code, route)
	bot := Request{User: "bot", Token: "valid", Device: 12, PerSecond: 40}
	code, route = g.Handle(bot, "us-east", "us-west")
	fmt.Printf("  a bot sending 40 per second -> %d %s\n", code, route)
	g.Drained["us-east"] = true
	code, route = g.Handle(ola, "us-east", "us-west")
	fmt.Printf("step 5: us-east drained -> %d %s\n", code, route)
	g.Drained["us-east"] = false

	fmt.Println("step 6: Playback API answers in 5 s, 50 requests per second, 200 threads")
	need := ThreadsNeeded(50, 5*time.Second)
	fmt.Printf("  threads needed = 50 * 5 s = %d > 200: every thread waits, every route stalls\n", need)

	fmt.Println("step 7: Zuul 2, timeout 1 s, limit 100 in flight per route")
	g.Latency["playback-api@us-east"] = 5 * time.Second
	g.InFlight["playback-api@us-east"] = ThreadsNeeded(50, 1*time.Second)
	code, route = g.Handle(ola, "us-east", "us-west")
	fmt.Printf("  playback in flight %d <= 100, call cut at 1 s -> %d %s\n",
		g.InFlight["playback-api@us-east"], code, route)
	code, route = g.Handle(ben, "us-east", "us-west")
	fmt.Printf("  other route still fine -> %d %s\n", code, route)
	g.InFlight["playback-api@us-east"] = 100
	code, route = g.Handle(ola, "us-east", "us-west")
	fmt.Printf("  at 100 in flight -> %d %s\n", code, route)
}
