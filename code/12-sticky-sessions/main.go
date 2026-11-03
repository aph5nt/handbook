// Sticky sessions: the story of the lesson 12-sticky-sessions.html, replayed.
// A balancer pins each agent to the copy that holds its session, one heavy
// agent overloads one copy, a deploy logs out 800 agents, and a shared
// session store makes a restart free.
package main

import (
	"fmt"
	"hash/fnv"
)

// Copy is one graph API copy. Mem holds sessions in its own memory.
type Copy struct {
	Name string
	Up   bool
	Load int // percent of one copy's capacity
	Mem  map[string]bool
}

// Request is one call from an agent, with its cookie if it has one.
type Request struct {
	Agent  string
	Cookie string // the copy name, set by the balancer at login
	Cost   int
}

// Sticky is a cookie-affinity balancer.
type Sticky struct{ Copies []*Copy }

func (b *Sticky) Route(r *Request) *Copy {
	if r.Cookie != "" {
		for _, c := range b.Copies {
			if c.Name == r.Cookie {
				return c // follow the cookie, whatever the load
			}
		}
	}
	c := LeastLoaded(b.Copies) // first request: pick once
	r.Cookie = c.Name          // and pin the agent to it
	return c
}

func LeastLoaded(cs []*Copy) *Copy {
	var best *Copy
	for _, c := range cs {
		if c.Up && (best == nil || c.Load < best.Load) {
			best = c
		}
	}
	return best
}

// IPHash is L4 affinity: the client IP address decides the copy.
func IPHash(ip string, cs []*Copy) *Copy {
	h := fnv.New32a()
	h.Write([]byte(ip))
	return cs[h.Sum32()%uint32(len(cs))]
}

// Handle serves a request: the session is in the copy's memory, or in the
// shared store when there is one.
func (c *Copy) Handle(r *Request, store map[string]bool) bool {
	c.Load += r.Cost
	if store != nil {
		return store[r.Agent]
	}
	return c.Mem[r.Agent] // false: "who are you?", the agent is logged out
}

// Restart wipes the copy's memory.
func (c *Copy) Restart() int {
	lost := len(c.Mem)
	c.Mem = map[string]bool{}
	return lost
}

// Soft is affinity as a hint: prefer the last copy unless it is down or busy.
func Soft(last *Copy, cs []*Copy, busy int) *Copy {
	if last != nil && last.Up && last.Load < busy {
		return last // warm cache
	}
	return LeastLoaded(cs)
}

func show(cs []*Copy) string {
	s := ""
	for _, c := range cs {
		s += fmt.Sprintf(" %s %d%%", c.Name, c.Load)
	}
	return s
}

func main() {
	fmt.Println("== pin: affinity at work")
	c1 := &Copy{Name: "copy 1", Up: true, Mem: map[string]bool{}}
	c2 := &Copy{Name: "copy 2", Up: false, Mem: map[string]bool{}}
	lb := &Sticky{Copies: []*Copy{c1, c2}}
	a7 := &Request{Agent: "agent 7", Cost: 40}
	got := lb.Route(a7)
	got.Mem[a7.Agent] = true
	got.Handle(a7, nil)
	fmt.Println("1 agent 7 logs in, no cookie: balancer picks", got.Name)
	fmt.Println("2 reply sets cookie:", a7.Cookie)
	a9 := &Request{Agent: "agent 9", Cost: 40}
	lb.Route(a9).Mem[a9.Agent] = true
	c1.Handle(a9, nil)
	fmt.Println("3 agent 7 ->", lb.Route(a7).Name, " agent 9 ->", lb.Route(a9).Name)
	c2.Up = true
	a3 := &Request{Agent: "agent 3", Cost: 20}
	got = lb.Route(a3)
	got.Mem[a3.Agent] = true
	got.Handle(a3, nil)
	fmt.Println("4 copy 2 starts, agent 3 logs in ->", got.Name, " loads:"+show(lb.Copies))
	batch := &Request{Agent: "agent 9", Cookie: a9.Cookie, Cost: 40}
	lb.Route(batch).Handle(batch, nil)
	fmt.Println("5 agent 9 batch follows its cookie, loads:" + show(lb.Copies))
	for _, who := range []string{"agent 7", "agent 9", "agent 3"} {
		fmt.Println("6 IP hash, office 203.0.113.7,", who, "->", IPHash("203.0.113.7", lb.Copies).Name)
	}

	fmt.Println("== crash: a restart, and the fix")
	c1 = &Copy{Name: "copy 1", Up: true, Mem: map[string]bool{}}
	c2 = &Copy{Name: "copy 2", Up: true, Mem: map[string]bool{}}
	for i := 0; i < 800; i++ {
		c := c1
		if i >= 600 {
			c = c2
		}
		c.Mem[fmt.Sprint("agent ", i)] = true
	}
	fmt.Println("1 sessions in memory: copy 1", len(c1.Mem), " copy 2", len(c2.Mem))
	out := c1.Restart()
	ok := c1.Handle(&Request{Agent: "agent 7"}, nil)
	fmt.Println("2 deploy restarts copy 1:", out, "logged out, agent 7 found:", ok)
	out += c2.Restart()
	fmt.Println("3 deploy restarts copy 2:", out, "logged out in all")

	store := map[string]bool{}
	for i := 0; i < 800; i++ {
		store[fmt.Sprint("agent ", i)] = true
	}
	cs := []*Copy{{Name: "copy 1", Up: true}, {Name: "copy 2", Up: true}}
	cs[0].Load, cs[1].Load = 50, 50 // any copy serves anyone: the balancer evens the load
	fmt.Println("4 sessions in the shared store:", len(store), " least loaded:"+show(cs))
	cs[0].Up = false
	lost := 0
	for i := 0; i < 800; i++ {
		if !LeastLoaded(cs).Handle(&Request{Agent: fmt.Sprint("agent ", i)}, store) {
			lost++
		}
	}
	fmt.Println("5 copy 1 restarts, 800 requests go to", LeastLoaded(cs).Name, "- logged out:", lost)
	cs[0].Up, cs[0].Load, cs[1].Load = true, 50, 50
	fmt.Println("6 soft affinity, last copy 1 at 50%:", Soft(cs[0], cs, 80).Name)
	cs[0].Load = 90
	fmt.Println("  last copy 1 at 90%, busy line 80%:", Soft(cs[0], cs, 80).Name)
	cs[0].Up = false
	fmt.Println("  last copy 1 down:", Soft(cs[0], cs, 80).Name)
}
