// Read replicas and replica lag: the story of the lesson 12-read-replicas.html,
// replayed. Dashboards move to two replicas, a backfill leaves one replica
// 40 s behind, and a lag monitor takes it out of the pool until it catches up.
package main

import "fmt"

const limit = 5.0 // seconds of lag a replica may have and stay in the pool

// Replica applies the primary's changes at its own rate.
type Replica struct {
	Name    string
	Applied int // rows applied so far
	Rate    int // rows it can apply per second
	InPool  bool
}

// Lag is seconds behind: 0.2 s of steady delay plus the rows it still owes,
// at the 1,000 rows per second the backfill writes.
func (r *Replica) Lag(written int) float64 {
	return 0.2 + float64(written-r.Applied)/1000
}

func (r *Replica) Tick(written int) {
	r.Applied = min(written, r.Applied+r.Rate)
}

// Router decides where each read goes.
type Router struct {
	Replicas []*Replica
	next     int
}

// Route sends a user who wrote in the last 5 s to the primary. Everyone else
// goes round the replicas in the pool. An empty pool falls back to the primary.
func (rt *Router) Route(sinceWrite float64) string {
	if sinceWrite < 5 {
		return "primary"
	}
	for range rt.Replicas {
		r := rt.Replicas[rt.next%len(rt.Replicas)]
		rt.next++
		if r.InPool {
			return r.Name
		}
	}
	return "primary"
}

// Monitor checks every replica on its own, never an average.
func (rt *Router) Monitor(written int) {
	for _, r := range rt.Replicas {
		r.InPool = r.Lag(written) <= limit
	}
}

// Load is percent busy: writes hit every node, dashboards split over replicas.
func Load(writes, dashboards, replicas int) (primary, each int) {
	if replicas == 0 {
		return writes + dashboards, 0
	}
	return writes, writes + dashboards/replicas
}

func main() {
	fmt.Println("== route: route reads by need (percent busy)")
	p, _ := Load(30, 60, 0)
	fmt.Println("1 one database: writes 30 + dashboards 60 =", p)
	fmt.Println("2 two replicas start, lag 0.2 s, reads not moved yet: primary still", p)
	p, e := Load(30, 60, 2)
	fmt.Printf("3 dashboards on 2 replicas: primary %d, each replica %d\n", p, e)
	rt := &Router{Replicas: []*Replica{{Name: "replica 1", InPool: true}, {Name: "replica 2", InPool: true}}}
	fmt.Println("4 user wrote 2 s ago ->", rt.Route(2), "  wrote 10 s ago ->", rt.Route(10))
	p, e = Load(30, 60, 4)
	fmt.Printf("5 four replicas: primary still %d, each replica %d\n", p, e)

	fmt.Println("== lag: a lag spike")
	r1 := &Replica{Name: "replica 1", Applied: -100, Rate: 1000, InPool: true}
	r2 := &Replica{Name: "replica 2", Applied: 0, Rate: 750, InPool: true}
	rt = &Router{Replicas: []*Replica{r1, r2}}
	written := 0
	for t := 1; t <= 160; t++ {
		written += 1000 // the backfill
		r1.Tick(written - 100)
		r2.Tick(written)
	}
	fmt.Printf("1 after 160 s of backfill: replica 1 lag %.1f s, replica 2 lag %.1f s\n", r1.Lag(written), r2.Lag(written))
	fmt.Printf("2 dashboard on replica 2 counts %d rows, the primary has %d: %d short, no error\n",
		r2.Applied, written, written-r2.Applied)
	rt.Monitor(written)
	fmt.Printf("3 monitor, limit %.0f s: replica 1 in pool %v, replica 2 in pool %v\n", limit, r1.InPool, r2.InPool)
	fmt.Println("4 next three dashboards:", rt.Route(60), rt.Route(60), rt.Route(60))
	for t := 161; ; t++ {
		r1.Tick(written)
		r2.Tick(written)
		rt.Monitor(written)
		if r2.InPool {
			fmt.Printf("5 backfill over, t=%d s: replica 2 lag %.1f s, back in pool\n", t, r2.Lag(written))
			break
		}
	}
	for r2.Applied < written {
		r2.Tick(written)
	}
	fmt.Printf("  fully caught up: replica 1 lag %.1f s, replica 2 lag %.1f s\n", r1.Lag(written), r2.Lag(written))
}
