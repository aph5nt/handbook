// Scale out: the story of the lesson 12-scale-out.html, replayed.
// An MCP proxy grows from one process to three stateless copies behind a
// balancer that uses "power of two choices", and an autoscaler with a
// cool-down. Then the hidden-state failure: sessions kept in one copy's RAM.
package main

import (
	"errors"
	"fmt"
	"hash/fnv"
	"time"
)

// Copy is one running proxy process.
type Copy struct {
	Name     string
	InFlight int
	Memory   map[string]string // hidden state: it dies with the process
}

// Balancer spreads agent queries over the copies.
type Balancer struct {
	Copies []*Copy
	Store  map[string]string // the shared store, outside every copy
	Sticky bool              // the sticky-session patch
}

// PickOfTwo is "power of two choices": of two random copies, take the less busy.
func PickOfTwo(c1, c2 *Copy) *Copy {
	if c1.InFlight <= c2.InFlight {
		return c1
	}
	return c2
}

func hash(s string) int {
	h := fnv.New32a()
	h.Write([]byte(s))
	return int(h.Sum32())
}

// Handle answers one query. shared=false is the hidden-state bug.
func (b *Balancer) Handle(agent string, c *Copy, shared bool) error {
	if b.Sticky {
		c = b.Copies[hash(agent)%len(b.Copies)] // the same copy every time
	}
	var ok bool
	if shared {
		_, ok = b.Store[agent] // any copy can read it
	} else {
		_, ok = c.Memory[agent] // only the copy that saw the login has it
	}
	if !ok {
		return errors.New("401 who are you?")
	}
	return nil
}

// Autoscaler adds or removes copies on queue depth, with a cool-down.
type Autoscaler struct {
	Copies     int
	LastChange time.Time
}

// Tick runs every 30 seconds.
func (a *Autoscaler) Tick(now time.Time, waiting int) (float64, string) {
	depth := float64(waiting) / float64(a.Copies)
	if now.Sub(a.LastChange) < 5*time.Minute {
		return depth, "cool-down: wait"
	}
	if depth > 20 {
		a.Copies++
		a.LastChange = now
		return depth, "add a copy"
	}
	if depth < 5 && a.Copies > 1 {
		a.Copies--
		a.LastChange = now
		return depth, "remove a copy"
	}
	return depth, "hold"
}

func main() {
	t0 := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)

	fmt.Println("== Grow: up, then out")
	one := &Autoscaler{Copies: 1, LastChange: t0.Add(-time.Hour)}
	d, _ := one.Tick(t0, 60)
	fmt.Printf("step 2: 1 copy, 60 waiting -> depth %.0f per copy, above 20\n", d)

	c1 := &Copy{Name: "copy 1", InFlight: 5}
	c2 := &Copy{Name: "copy 2", InFlight: 12}
	c3 := &Copy{Name: "copy 3", InFlight: 4}
	fmt.Printf("step 6: random pair copy 2 (%d in flight) and copy 3 (%d) -> send to %s\n",
		c2.InFlight, c3.InFlight, PickOfTwo(c2, c3).Name)

	as := &Autoscaler{Copies: 3, LastChange: t0.Add(-2 * time.Minute)}
	d, verdict := as.Tick(t0, 13)
	fmt.Printf("step 7: 13 waiting over 3 copies -> depth %.1f, below 5, last change 2 min ago -> %s\n", d, verdict)
	d, verdict = as.Tick(t0.Add(3*time.Minute), 13)
	fmt.Printf("        3 minutes later, depth %.1f -> %s, copies now %d\n", d, verdict, as.Copies)

	fmt.Println("== Hidden state: the failure")
	c1 = &Copy{Name: "copy 1", InFlight: 9, Memory: map[string]string{"agent 7": "session"}}
	c2 = &Copy{Name: "copy 2", InFlight: 0, Memory: map[string]string{}}
	b := &Balancer{Copies: []*Copy{c1, c2}, Store: map[string]string{}}
	pick := PickOfTwo(c1, c2)
	fmt.Printf("step 2: copy 1 has %d in flight, copy 2 has %d -> %s: %v\n", c1.InFlight, c2.InFlight, pick.Name, b.Handle("agent 7", pick, false))

	b.Copies[0].Memory = map[string]string{} // copy 1 crashed: its RAM is gone
	fmt.Printf("step 4: copy 1 crashed and restarted with empty RAM: %v\n", b.Handle("agent 7", c1, false))

	b.Store["agent 7"] = "session" // the fix: the session lives in the shared store
	fmt.Printf("step 5: session in the shared store, served by %s: error=%v\n", c2.Name, b.Handle("agent 7", c2, true))
}
