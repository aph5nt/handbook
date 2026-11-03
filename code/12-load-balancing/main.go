// Load balancing: the story of the lesson 12-load-balancing.html, replayed.
// A balancer in front of three graph copies tries five picking rules, then an
// L4 balancer pins one HTTP/2 connection to one copy.
package main

import (
	"fmt"
	"hash/fnv"
)

// Copy is one graph copy with its open requests.
type Copy struct {
	Name   string
	Open   int
	Weight int
	cur    int // running score for smooth weighted round robin
}

// RoundRobin takes turns, blind to load.
type RoundRobin struct{ next int }

func (r *RoundRobin) Pick(cs []*Copy) *Copy {
	c := cs[r.next%len(cs)]
	r.next++
	return c
}

// LeastConnections picks the copy with the fewest open requests.
func LeastConnections(cs []*Copy) *Copy {
	best := cs[0]
	for _, c := range cs[1:] {
		if c.Open < best.Open {
			best = c
		}
	}
	return best
}

// Weighted is smooth weighted round robin: each pick, add each weight to its
// score, take the highest score, then subtract the total weight from it.
func Weighted(cs []*Copy) *Copy {
	total := 0
	var best *Copy
	for _, c := range cs {
		c.cur += c.Weight
		total += c.Weight
		if best == nil || c.cur > best.cur {
			best = c
		}
	}
	best.cur -= total
	return best
}

// PowerOfTwo compares two random copies and takes the less busy one.
func PowerOfTwo(a, b *Copy) *Copy {
	if a.Open <= b.Open {
		return a
	}
	return b
}

// ByHash sends the same key to the same copy every time.
func ByHash(cs []*Copy, key string) *Copy {
	h := fnv.New32a()
	h.Write([]byte(key))
	return cs[h.Sum32()%uint32(len(cs))]
}

func three(open ...int) []*Copy {
	cs := []*Copy{{Name: "copy 1"}, {Name: "copy 2"}, {Name: "copy 3"}}
	for i, o := range open {
		cs[i].Open = o
	}
	return cs
}

// L4Connect picks once, at connect time. Every call rides that connection.
func L4Connect(cs []*Copy, calls int) *Copy {
	conn := LeastConnections(cs)
	conn.Open += calls
	return conn
}

// L7Calls picks again for every call.
func L7Calls(cs []*Copy, calls int) {
	for i := 0; i < calls; i++ {
		LeastConnections(cs).Open++
	}
}

// Route is what L7 can do because it reads the request: pick copies by path.
func Route(cs []*Copy, path string) []*Copy {
	if path == "/health" {
		return cs[2:] // copy 3
	}
	return cs[:2] // /graph goes to copies 1 and 2
}

func main() {
	fmt.Println("== The picking rules")
	rr := &RoundRobin{}
	cs := three(0, 0, 0)
	for i := 0; i < 4; i++ {
		fmt.Printf("step 1: round robin call %d -> %s\n", i+1, rr.Pick(cs).Name)
	}
	cs = three(8, 2, 1)
	rr = &RoundRobin{next: 0}
	fmt.Printf("step 2: open %d/%d/%d, round robin -> %s (its turn)\n", cs[0].Open, cs[1].Open, cs[2].Open, rr.Pick(cs).Name)
	c := LeastConnections(cs)
	fmt.Printf("step 3: open %d/%d/%d, least connections -> %s\n", cs[0].Open, cs[1].Open, cs[2].Open, c.Name)
	cs = three(0, 0, 0)
	cs[0].Weight, cs[1].Weight, cs[2].Weight = 1, 2, 1
	got := map[string]int{}
	for i := 0; i < 4; i++ {
		got[Weighted(cs).Name]++
	}
	fmt.Printf("step 4: weights 1/2/1, 4 calls -> copy 1: %d, copy 2: %d, copy 3: %d\n", got["copy 1"], got["copy 2"], got["copy 3"])
	cs = three(7, 3, 2)
	fmt.Printf("step 5: random pair copy 1 (%d open) and copy 3 (%d open) -> %s\n", cs[0].Open, cs[2].Open, PowerOfTwo(cs[0], cs[2]).Name)
	cs = three(0, 0, 0)
	for i := 0; i < 3; i++ {
		fmt.Printf("step 6: hash(0x4a) mod 3 -> %s\n", ByHash(cs, "0x4a").Name)
	}

	fmt.Println("== L4 and one long connection")
	cs = three(0, 0, 0)
	conn := L4Connect(cs, 1000)
	fmt.Printf("step 2: 1000 calls on one HTTP/2 connection -> %s has %d, copy 2 has %d, copy 3 has %d\n", conn.Name, cs[0].Open, cs[1].Open, cs[2].Open)
	cs = three(0, 0, 0)
	L7Calls(cs, 9)
	fmt.Printf("step 4: L7, 9 calls, least connections per call -> %d/%d/%d\n", cs[0].Open, cs[1].Open, cs[2].Open)
	fmt.Printf("step 5: L7 routes /graph to %s and %s, /health to %s\n", Route(cs, "/graph")[0].Name, Route(cs, "/graph")[1].Name, Route(cs, "/health")[0].Name)
}
