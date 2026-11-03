// Layer 4 balances connections. Layer 7 balances requests. A gRPC client
// keeps one connection, so at layer 4 every request lands on one pod.
package main

import "fmt"

// RoundRobin hands out the next backend.
type RoundRobin struct {
	Pool []string
	next int
}

func (r *RoundRobin) Pick() string {
	b := r.Pool[r.next%len(r.Pool)]
	r.next++
	return b
}

// L4 picks a backend once, when the connection opens.
func L4(r *RoundRobin) func() string {
	backend := r.Pick()
	return func() string { return backend } // every request on this connection
}

// L7 reads the path, then picks a healthy pod per request.
type L7 struct {
	Pools   map[string]*RoundRobin
	Healthy map[string]bool
}

func (l *L7) Route(path string) string {
	pool := l.Pools[path]
	for range pool.Pool {
		if p := pool.Pick(); l.Healthy[p] {
			return p
		}
	}
	return "503"
}

func count(n int, pick func() string) map[string]int {
	c := map[string]int{}
	for i := 0; i < n; i++ {
		c[pick()]++
	}
	return c
}

func main() {
	ingress := &RoundRobin{Pool: []string{"ingress-1", "ingress-2"}}
	fmt.Println("connection 1 ->", ingress.Pick(), " connection 2 ->", ingress.Pick())

	l7 := &L7{
		Pools: map[string]*RoundRobin{
			"/balance": {Pool: []string{"api-1", "api-2"}},
			"/graph":   {Pool: []string{"graph-1", "graph-2"}},
		},
		Healthy: map[string]bool{"api-1": true, "api-2": true, "graph-1": true, "graph-2": true},
	}
	fmt.Println("GET /balance ->", l7.Route("/balance"), " GET /graph ->", l7.Route("/graph"))
	l7.Healthy["graph-2"] = false
	fmt.Println("graph-2 fails its health check, 4 x /graph:", count(4, func() string { return l7.Route("/graph") }))

	api := &RoundRobin{Pool: []string{"api-1", "api-2", "api-3"}}
	conn := L4(api)
	fmt.Println("gRPC, one connection, layer 4, 3000 requests:", count(3000, conn))
	api.Pool = append(api.Pool, "api-4", "api-5")
	fmt.Println("the autoscaler adds 2 pods, same connection:  ", count(3000, conn))
	perRequest := &RoundRobin{Pool: api.Pool}
	fmt.Println("layer 7 or the client, per request:           ", count(3000, perRequest.Pick))
}
