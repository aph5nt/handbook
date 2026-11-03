// Service discovery: the story of the lesson 12-service-discovery.html, replayed.
// Three price copies register with a registry, a health check removes a dead
// one, a cached client sends calls to a dead copy, and the three-layer fix
// closes the gap.
package main

import (
	"fmt"
	"slices"
	"strings"
)

// Copy is one running price service copy.
type Copy struct {
	Addr    string
	Service string
	Alive   bool
	Ready   bool
	Open    int
}

// Entry is one row in the registry.
type Entry struct {
	Addr   string
	Misses int
}

// Registry is a small database of who runs where.
type Registry struct {
	Up      bool
	Rows    map[string][]*Entry
	Checks  bool // health checks on or off
	MaxMiss int  // missed checks in a row before removal
}

func (r *Registry) Register(service, addr string) {
	r.Rows[service] = append(r.Rows[service], &Entry{Addr: addr})
}

func (r *Registry) Deregister(service, addr string) {
	r.Rows[service] = slices.DeleteFunc(r.Rows[service], func(e *Entry) bool { return e.Addr == addr })
}

// HealthCheck runs every 5 s: call each copy, drop it after MaxMiss misses.
func (r *Registry) HealthCheck(service string, net map[string]*Copy) {
	if !r.Checks {
		return // turned off: dead copies stay listed
	}
	r.Rows[service] = slices.DeleteFunc(r.Rows[service], func(e *Entry) bool {
		if c := net[e.Addr]; c != nil && c.Alive && c.Service == service {
			e.Misses = 0 // answered
		} else {
			e.Misses++ // no answer
		}
		return e.Misses >= r.MaxMiss
	})
}

func (r *Registry) Lookup(service string) ([]string, bool) {
	if !r.Up {
		return nil, false
	}
	out := []string{}
	for _, e := range r.Rows[service] {
		out = append(out, e.Addr)
	}
	return out, true
}

// Client is the graph API with a cached copy of the list.
type Client struct {
	Cache    []string
	CachedAt int
	TTL      int // seconds
	Retry    bool
	next     int
}

func (c *Client) List(r *Registry, now int) []string {
	if c.Cache != nil && now-c.CachedAt < c.TTL {
		return c.Cache // trust the cache
	}
	if l, ok := r.Lookup("price"); ok {
		c.Cache, c.CachedAt = l, now
	}
	return c.Cache // registry down: keep the last known list
}

// Call sends one price query round robin over the list.
func (c *Client) Call(r *Registry, net map[string]*Copy, now int) string {
	list := c.List(r, now)
	for try := 0; try < len(list); try++ {
		addr := list[c.next%len(list)]
		c.next++
		cp := net[addr]
		switch {
		case cp == nil || !cp.Alive:
			if !c.Retry {
				return "timeout (2 s) at " + addr
			}
			c.CachedAt = now - c.TTL // refused: expire the cache
			list = c.List(r, now)    // refresh, then try another copy
		case cp.Service != "price":
			return "wrong service at " + addr + ": " + cp.Service
		default:
			return "ok from " + addr
		}
	}
	return "no copy"
}

// PowerOfTwo picks the less busy of two sampled copies.
func PowerOfTwo(a, b *Copy) *Copy {
	if a.Open <= b.Open {
		return a
	}
	return b
}

// Endpoints is what a Kubernetes Service forwards to: ready pods only.
func Endpoints(pods []*Copy) []string {
	out := []string{}
	for _, p := range pods {
		if p.Ready {
			out = append(out, p.Addr)
		}
	}
	return out
}

func newWorld() (*Registry, map[string]*Copy, []*Copy) {
	r := &Registry{Up: true, Rows: map[string][]*Entry{}, Checks: true, MaxMiss: 2}
	net := map[string]*Copy{}
	pods := []*Copy{}
	for i, open := range []int{4, 1, 3} {
		c := &Copy{Addr: fmt.Sprintf("10.0.0.1%d", i+1), Service: "price", Alive: true, Ready: true, Open: open}
		net[c.Addr], pods = c, append(pods, c)
		r.Register("price", c.Addr) // each copy registers itself at start
	}
	return r, net, pods
}

func calls(cl *Client, r *Registry, net map[string]*Copy, now, n int) {
	bad := 0
	for k := 0; k < n; k++ {
		res := cl.Call(r, net, now)
		if !strings.HasPrefix(res, "ok") {
			bad++
			if bad == 1 {
				fmt.Println("    first failure:", res)
			}
		}
	}
	fmt.Printf("    %d calls, %d failed\n", n, bad)
}

func main() {
	fmt.Println("== found: client-side and server-side")
	typed := []string{"10.0.0.11", "10.0.0.12", "10.0.0.13"}
	after := map[string]bool{"10.0.0.21": true, "10.0.0.22": true, "10.0.0.23": true}
	hit := 0
	for _, a := range typed {
		if after[a] {
			hit++
		}
	}
	fmt.Printf("1 typed list after a deploy: %d of 3 addresses still alive\n", hit)

	r, net, pods := newWorld()
	l, _ := r.Lookup("price")
	fmt.Println("2 registered:", l)
	r.HealthCheck("price", net)
	l, _ = r.Lookup("price")
	fmt.Println("3 health check at 5 s: all answer, list", l)
	got := PowerOfTwo(pods[1], pods[2]) // sampled copy 2 (1 open) and copy 3 (3 open)
	fmt.Printf("4 client-side: power of two over copy 2 (%d open) and copy 3 (%d open) picks %s\n",
		pods[1].Open, pods[2].Open, got.Addr)
	pods[2].Alive = false // copy 3 crashes at 12 s
	for t := 15; t <= 20; t += 5 {
		r.HealthCheck("price", net)
		l, _ = r.Lookup("price")
		fmt.Printf("5 check at %d s: copy 3 missed %d, list %v\n", t, (t-15)/5+1, l)
	}
	pods[2].Ready = false
	fmt.Println("6 Kubernetes endpoints, ready pods only:", Endpoints(pods))

	fmt.Println("== stale: a dead copy stays listed")
	r, net, pods = newWorld()
	cl := &Client{TTL: 300}
	cl.List(r, 0)
	fmt.Println("1 client caches", cl.Cache, "for", cl.TTL, "s")
	pods[2].Alive = false // killed at 60 s, no graceful shutdown, no deregister
	fmt.Println("2 copy 3 killed at 60 s, never deregisters")
	r.Checks = false // turned off months ago
	r.HealthCheck("price", net)
	l, _ = r.Lookup("price")
	fmt.Println("3 checks off, registry still lists", l)
	calls(cl, r, net, 90, 9)
	net["10.0.0.13"] = &Copy{Addr: "10.0.0.13", Service: "labels", Alive: true}
	fmt.Println("4 at 660 s 10.0.0.13 is reused by the labels service")
	cl.Cache, cl.CachedAt = []string{"10.0.0.11", "10.0.0.12", "10.0.0.13"}, 600
	calls(cl, r, net, 660, 9)

	fmt.Println("5 the fix: checks every 5 s, deregister on shutdown, cache 10 s, retry")
	r, net, pods = newWorld()
	cl = &Client{TTL: 10, Retry: true}
	cl.List(r, 0)
	pods[2].Alive = false // killed: the shutdown hook did not run
	calls(cl, r, net, 5, 9)
	for t := 5; t <= 10; t += 5 {
		r.HealthCheck("price", net)
	}
	l, _ = r.Lookup("price")
	fmt.Println("    after 2 missed checks (10 s) registry lists", l)
	cl.List(r, 15)
	fmt.Println("    client refreshes at 15 s:", cl.Cache)
	r2, _, _ := newWorld()
	r2.Deregister("price", "10.0.0.13") // graceful: the shutdown hook runs first
	l, _ = r2.Lookup("price")
	fmt.Println("    graceful stop deregisters at once:", l)

	r.Up = false
	fmt.Println("6 registry down at 30 s, client keeps", cl.List(r, 30))
	calls(cl, r, net, 30, 9)
}
