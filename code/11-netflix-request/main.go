// Command netflix replays the Netflix play-request lesson: Zuul, a cached
// Eureka list, three calls with timeouts and a breaker per dependency, the
// fallback row, video from Open Connect, and a Chaos Monkey run that finds
// a client with a fixed address.
package main

import "fmt"

type Dep struct {
	Name      string
	LatencyMs int
	Fails     int
	Open      bool
}

func Play(user string, deps []*Dep) map[string]string {
	answer := map[string]string{}
	for _, d := range deps {
		if d.Open {
			answer[d.Name] = "fallback"
			continue
		}
		if d.LatencyMs > 1000 {
			d.Fails++
			answer[d.Name] = "fallback"
			if d.Fails >= 5 {
				d.Open = true
			}
			continue
		}
		answer[d.Name] = "ok"
	}
	answer["video"] = "Open Connect box"
	return answer
}

func Zuul(token string) bool                           { return token == "valid" }
func ChaosMonkey(alive map[string]bool, victim string) { alive[victim] = false }

func CallAPI(copies []string, alive map[string]bool) (tried []string, ok bool) {
	for i := 0; i < len(copies) && i < 2; i++ {
		tried = append(tried, copies[i])
		if alive[copies[i]] {
			return tried, true
		}
	}
	return tried, false
}

func Heal(registry []string, alive map[string]bool, next string) (live []string) {
	for _, c := range registry {
		if alive[c] {
			live = append(live, c)
		}
	}
	if len(live) < 3 && next != "" {
		alive[next] = true
		live = append(live, next)
	}
	return live
}

var fixedAddress = []string{"api-2"} // BUG: never asks Eureka

func main() {
	deps := []*Dep{{Name: "profile", LatencyMs: 20}, {Name: "licence", LatencyMs: 40},
		{Name: "recommendations", LatencyMs: 4000}}
	fmt.Println("play tab")
	fmt.Println("step 1-3: Zuul routes; the Playback API reads its cached Eureka list")
	for v := 1; v <= 5; v++ {
		a := Play(fmt.Sprintf("viewer%d", v), deps)
		fmt.Printf("step 4-5: viewer %d: recommendations 4000 ms > 1000 ms -> %s, breaker fails %d, open %v\n",
			v, a["recommendations"], deps[2].Fails, deps[2].Open)
	}
	a := Play("Ola", deps)
	fmt.Printf("step 6: Ola: profile %s, licence %s, recommendations %s (no call, breaker open)\n",
		a["profile"], a["licence"], a["recommendations"])
	fmt.Println("step 7: video from", a["video"])

	fmt.Println("chaos tab")
	alive := map[string]bool{"api-1": true, "api-2": true, "api-3": true}
	registry := []string{"api-1", "api-2", "api-3"}
	fmt.Println("step 1: Zuul lets Ola in:", Zuul("valid"), "alive copies", registry)
	ChaosMonkey(alive, "api-2")
	fmt.Println("step 2: Chaos Monkey kills api-2 at 11:00")
	tried, ok := CallAPI([]string{"api-2", "api-3"}, alive)
	fmt.Println("step 3: request in flight to api-2 -> tried", tried, "ok", ok)
	registry = Heal(registry, alive, "")
	fmt.Println("step 4: Eureka drops api-2 ->", registry)
	registry = Heal(registry, alive, "api-4")
	fmt.Println("step 5: 2 < 3, autoscaler starts api-4 ->", registry)
	tried, ok = CallAPI(fixedAddress, alive)
	fmt.Println("step 6: old client with fixed address -> tried", tried, "ok", ok, "on every call")
}
