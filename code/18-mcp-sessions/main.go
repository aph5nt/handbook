// Stateful and stateless servers behind a balancer. A session lives in one
// replica's memory, so routing must follow Mcp-Session-Id. Run: go run .
package main

import "fmt"

type Replica struct {
	Name     string
	stateful bool
	sessions map[string]bool
}

func (r *Replica) initialize(id string) string {
	r.sessions[id] = true
	return "Mcp-Session-Id: " + id + " from " + r.Name
}

func (r *Replica) post(sid string) string {
	if r.stateful && !r.sessions[sid] {
		return r.Name + ": 404 session not found"
	}
	return r.Name + ": 200 result"
}

func (r *Replica) restart() { r.sessions = map[string]bool{} }

// Balancer picks a replica. sticky follows the Mcp-Session-Id header.
type Balancer struct {
	replicas []*Replica
	next     int
	sticky   map[string]*Replica
}

func (b *Balancer) roundRobin() *Replica {
	r := b.replicas[b.next%len(b.replicas)]
	b.next++
	return r
}

func (b *Balancer) route(sid string) *Replica {
	if r, ok := b.sticky[sid]; ok {
		return r
	}
	return b.roundRobin()
}

func newReplicas(stateful bool) (*Replica, *Replica) {
	return &Replica{"replica 1", stateful, map[string]bool{}}, &Replica{"replica 2", stateful, map[string]bool{}}
}

func main() {
	a, c := newReplicas(false)
	b := &Balancer{replicas: []*Replica{a, c}, sticky: map[string]*Replica{}}
	fmt.Println("0 stateless:", b.route("").post(""), "|", b.route("").post(""))

	r1, r2 := newReplicas(true)
	b = &Balancer{replicas: []*Replica{r1, r2}, sticky: map[string]*Replica{}}
	fmt.Println("1", b.route("").initialize("s-7f3"))
	fmt.Println("2 round robin:", b.route("s-7f3").post("s-7f3"))

	b.sticky["s-7f3"] = r1 // route by the header
	fmt.Println("3 sticky:", b.route("s-7f3").post("s-7f3"))

	r1.restart()
	fmt.Println("4 after a restart:", b.route("s-7f3").post("s-7f3"))
	fmt.Println("4 new", b.route("s-7f3").initialize("s-9a1"))
	b.sticky["s-9a1"] = r1
	fmt.Println("4", b.route("s-9a1").post("s-9a1"), "then DELETE /mcp when done")
}
